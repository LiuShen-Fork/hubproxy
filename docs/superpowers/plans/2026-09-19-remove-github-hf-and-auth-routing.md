# 移除 GitHub/HF 加速 + 认证路由重构 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 删除 GitHub / Hugging Face 加速能力与离线镜像前端入口，重构顶部导航与注册/登录路由，并修复 OAuth 自动建号用户永不设密码的安全问题。

**Architecture:** 后端先收敛 URL 面——删掉 `/gh` `/hf` 路由与 GitHub 代理兜底，让 `NoRoute` 统一返回 404 JSON；再给 `FeatureToggles` 瘦身并让 OAuth 建号用户落回强制改密流程。前端随后删除 GitHub/HF 与离线镜像的页面与死代码，最后用一个 `AuthPage.vue` 同时承载 `/login` 与 `/register`，导航依据登录态在两套菜单间切换。

**Tech Stack:** Go 1.26 + Gin（后端）、SQLite / modernc.org/sqlite、Vue 3 + Vite + TypeScript + vue-router（前端）、`go test` 与 `vue-tsc` 作为验证手段。

**Spec:** `docs/superpowers/specs/2026-09-19-remove-github-hf-and-auth-routing-design.md`

## Global Constraints

- 所有改动在独立分支上完成，不要直接提交到 `main`。
- **Docker 拉取链路一行不动**：`/v2/*`、`/:token/v2/*`、`/token`、`NormalizeMirrorTokenPath`、`stripUserAccessToken`、`handleRegistryRequest`、`handleMultiRegistryRequest` 及其下游全部保持原样。
- **离线打包后端 API 保留**：`/api/image/download`、`/api/image/info`、`/api/image/batch` 与 `InitImageTarRoutes` 不动。
- `/TOKEN` 形式的浏览器误访问必须继续返回 404 且带 `X-Robots-Tag: noindex` 与 `Cache-Control: no-store`（防止个人令牌被搜索引擎索引）。
- 不做历史数据迁移：`pull_sessions` 中既有的 `category = 'github'/'huggingface'` 记录不清理、不兼容。
- 不保留 `/admin/login` 及其重定向。
- 前端没有测试框架，前端任务的验证手段是 `cd web && npm run build`（含 `vue-tsc` 类型检查）；不要引入 vitest 或其它测试依赖。
- 后端每个任务结束时跑 `cd src && go test ./...`。

  **验收标准是「相对基线无新增失败」，不是「全绿」。** 基线（分支起点 `9d3adbe`，已用干净工作树核实）本身就有 8 个既有失败，全部位于本次改造范围之外：

  | 包 | 失败用例 | 既有失败原因 |
  |---|---|---|
  | `hubproxy` | `TestReadyRoute` | 断言 `service == "hubproxy"`，而 fork 后 `initHealthRoutes` 返回 `"qingyu-mirror"`，断言未同步 |
  | `hubproxy` | `TestSingleImageDownloadPrepareReturnsURL`、`TestBatchImageDownloadPrepareReturnsURL`、`TestBatchImageDownloadRejectsTooManyImages` | 测试夹具未初始化 `db.GlobalRuntime`，零值使 `OfflineImage` 为 false，被离线下载开关中间件拦成 403 |
  | `hubproxy` | `TestDockerV2PingAndInvalidPath` | 同上，`PublicMirror` 零值为 false，被公共镜像开关拦成 403 |
  | `hubproxy/handlers` | `TestResolveAuthHost`、`TestBuildDockerAuthURL`、`TestRewriteAuthHeader` | 测试夹具未加载 Registry 配置，`GetRegistries()` 返回空，`resolveAuthHost` 拿不到 AuthHost |

  **不要在本计划内修复这 8 个失败**——它们与 GitHub/HF 移除无关，修它们会把这个聚焦分支变成混合改动。每个任务只需证明：自己的改动没有引入**新的**失败。做法是先跑一次基线，比对失败集合是否与上表完全一致。

  上表之外的任何失败都是本任务引入的回归，必须修掉。
- 面向用户的文案一律用中文，与既有风格保持一致。

---

### Task 1: 后端 NoRoute 统一 404 并删除 GitHub 代理

删除 GitHub 代理后，`NoRoute` 不再有兜底目标，所有未匹配路径必须收敛为 404 JSON。这一步同时删除四个代理路由和一段死中间件。

**Files:**
- Delete: `src/handlers/github.go`
- Delete: `src/handlers/content_proxy.go`
- Delete: `src/handlers/github_test.go`
- Modify: `src/main.go`
- Test: `src/main_test.go`

**Interfaces:**
- Consumes: 无
- Produces: `buildRouter(cfg *config.AppConfig) *gin.Engine` 行为变更——未匹配的 `/api/*` 返回 404 JSON 且 `code` 为 `NOT_FOUND`；未匹配的其他路径返回 404 JSON 且 `error` 为 `页面不存在`。后续任务不再依赖 `handlers.GitHubProxyHandler`、`handlers.ProxyGitHubRequest`、`handlers.CheckGitHubURL`、`handlers.contentProxy*` 中的任何符号。

- [ ] **Step 1: 建分支**

```bash
cd e:/Programming/HTML/hubproxy
git checkout -b feat/remove-github-hf-and-auth-routing
```

- [ ] **Step 2: 修改 main_test.go 里过时的断言，并新增 404 断言**

`TestGitHubNoRouteRejectsUnsupportedHost` 断言 `/https://example.com/file.zip` 返回 403，那是 GitHub 代理的行为，删除实现后该断言必然失败。删掉它，换成下面四个新用例。

在 `src/main_test.go` 中**删除**这整个函数：

```go
func TestGitHubNoRouteRejectsUnsupportedHost(t *testing.T) {
	router := newTestRouter(t, "")

	w := performRequest(router, http.MethodGet, "/https://example.com/file.zip", "")
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body=%s", w.Code, w.Body.String())
	}
}
```

在同文件末尾**新增**：

```go
func TestRemovedAccelerationPathsReturnNotFound(t *testing.T) {
	router := newTestRouter(t, "")

	for _, path := range []string{
		"/gh/owner/repo/raw/main/a.txt",
		"/hf/owner/model/resolve/main/x.bin",
		"/https://example.com/file.zip",
	} {
		w := performRequest(router, http.MethodGet, path, "")
		if w.Code != http.StatusNotFound {
			t.Fatalf("%s status = %d, want 404; body=%s", path, w.Code, w.Body.String())
		}
	}
}

func TestUnmatchedPathReturnsJSONNotFound(t *testing.T) {
	router := newTestRouter(t, "")

	w := performRequest(router, http.MethodGet, "/nonexistent", "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body=%s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Fatalf("content-type = %q, want application/json", ct)
	}
	var got map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("body is not JSON: %v; body=%s", err, w.Body.String())
	}
	if got["error"] != "页面不存在" || got["code"] != "NOT_FOUND" {
		t.Fatalf("unexpected body: %#v", got)
	}
}

func TestUnmatchedAPIPathReturnsJSONNotFound(t *testing.T) {
	router := newTestRouter(t, "")

	w := performRequest(router, http.MethodGet, "/api/nonexistent", "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body=%s", w.Code, w.Body.String())
	}
	var got map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("body is not JSON: %v; body=%s", err, w.Body.String())
	}
	if got["code"] != "NOT_FOUND" {
		t.Fatalf("unexpected body: %#v", got)
	}
}

func TestTokenBrowsePathKeepsNoindex(t *testing.T) {
	router := newTestRouter(t, "")

	w := performRequest(router, http.MethodGet, "/Ab12Cd34", "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body=%s", w.Code, w.Body.String())
	}
	if tag := w.Header().Get("X-Robots-Tag"); !strings.Contains(tag, "noindex") {
		t.Fatalf("X-Robots-Tag = %q, want noindex", tag)
	}
}
```

- [ ] **Step 3: 运行测试，确认新增用例失败**

Run: `cd src && go test ./... -run 'TestRemovedAccelerationPaths|TestUnmatchedPath|TestUnmatchedAPI|TestTokenBrowsePath' -v`
Expected: FAIL。`TestRemovedAccelerationPathsReturnNotFound` 会在 `/gh/...` 上拿到 403（GitHub 代理对非法主机返回 403）或非 404；`TestUnmatchedPathReturnsJSONNotFound` 会拿到纯文本 `无效输入` 而不是 JSON。

- [ ] **Step 4: 删除 GitHub 代理实现文件**

```bash
cd e:/Programming/HTML/hubproxy
git rm src/handlers/github.go src/handlers/content_proxy.go src/handlers/github_test.go
```

- [ ] **Step 5: 重写 src/main.go 的 buildRouter**

在 `src/main.go` 中**删除**这段死中间件（它只写入 `api_request`，全项目无任何读取方）：

```go
	// Block API paths from ever falling into GitHub proxy (must be early)
	router.Use(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.Set("api_request", true)
		}
		c.Next()
		// If an /api request ended with no response written and not aborted by a handler,
		// Gin will still hit NoRoute for unmatched routes — handled below.
	})
```

**删除**这四行路由注册：

```go
	// Token-scoped content acceleration. Public /gh and /hf are allowed only
	// when the public mirror switch is enabled.
	router.Any("/gh/*path", handlers.ProxyGitHubPublicPath)
	router.Any("/hf/*path", handlers.ProxyHuggingFacePublicPath)
	router.Any("/:token/gh/*path", handlers.ProxyGitHubTokenPath)
	router.Any("/:token/hf/*path", handlers.ProxyHuggingFaceTokenPath)
```

把整个 `NoRoute` 块**替换**为：

```go
	// 未匹配路由一律 404 JSON。GitHub 代理兜底已移除，不再有任何 catch-all 转发。
	router.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{
				"error":   "接口不存在。若刚升级，请完全停止旧进程后用最新代码重启（go run .）",
				"code":    "NOT_FOUND",
				"path":    path,
				"version": Version,
			})
			return
		}
		// 浏览器/爬虫误访问 /TOKEN 或 /TOKEN/...（/TOKEN/v2 已注册，不会走到这里）：
		// 返回 404 JSON + noindex，避免个人令牌被搜索引擎索引。
		trim := strings.TrimPrefix(path, "/")
		parts := strings.SplitN(trim, "/", 3)
		if len(parts) >= 1 && isAlnumToken(parts[0]) {
			if len(parts) == 1 || (len(parts) >= 2 && parts[1] != "v2" && parts[1] != "token") {
				c.Header("X-Robots-Tag", "noindex, nofollow, noarchive")
				c.Header("Cache-Control", "no-store")
				c.JSON(http.StatusNotFound, gin.H{
					"error": "页面不存在",
					"code":  "NOT_FOUND",
					"hint":  "此路径仅用于 Docker 镜像拉取，请勿在浏览器中打开",
				})
				return
			}
		}
		c.JSON(http.StatusNotFound, gin.H{
			"error": "页面不存在",
			"code":  "NOT_FOUND",
		})
	})
```

- [ ] **Step 6: 清理 pulltrack.go 中失去调用方的死代码**

删除 GitHub 代理后，`src/handlers/pulltrack.go` 中有两个函数不再有任何调用方：

- `trackContentPull`（原第 84 行）—— 唯一调用方是已删除的 `content_proxy.go:120`
- `recordContentBytes`（原第 139 行）—— 唯一调用方是已删除的 `github.go:167` 与 `github.go:193`

删除这两个函数。**保留**同文件中仍被 Docker 链路使用的：`countingWriter`、`trackDockerPull`、`recordPullBytes`、`CheckPullQuota`、`globalLimiterExempt`。

删除后先自查一遍：

```bash
cd e:/Programming/HTML/hubproxy
grep -rn "trackContentPull\|recordContentBytes" src --include=*.go
```

Expected: 无输出。

- [ ] **Step 7: 运行测试，确认全部通过**

Run: `cd src && go test ./...`
Expected: PASS。编译期若报 `handlers.GitHubProxyHandler undefined` 之类的错误，说明 `main.go` 里还残留引用，回到 Step 5 清理。

- [ ] **Step 8: 提交**

```bash
cd e:/Programming/HTML/hubproxy
git add src/main.go src/main_test.go src/handlers/pulltrack.go
git commit -m "refactor(proxy): drop github/huggingface acceleration and unify NoRoute to 404"
```

---

### Task 2: FeatureToggles 瘦身并删除 CheckGitHubAccess

`FeatureToggles` 里残留的 `GitHub` / `HuggingFace` 字段已无实现支撑，公开配置也不再应该暴露它们。旧 setting JSON 里的多余键在反序列化时会被自动忽略，因此不需要数据迁移。

**Files:**
- Modify: `src/db/settings.go`
- Modify: `src/handlers/auth.go`
- Modify: `src/utils/access_control.go`
- Modify: `src/utils/access_control_test.go`
- Test: `src/db/settings_test.go`（新建）

**Interfaces:**
- Consumes: 无
- Produces: `db.FeatureToggles` 结构体字段收敛为 `DockerHub` / `ImageSearch` / `OfflineImage` / `PublicMirror` / `RequireUserToken`，JSON tag 分别为 `docker_hub` / `image_search` / `offline_image` / `public_mirror` / `require_user_token`。`utils.AccessController.CheckGitHubAccess` 不再存在。

- [ ] **Step 1: 新建 src/db/settings_test.go，先写失败测试**

```go
package db

import (
	"encoding/json"
	"testing"
)

func TestDefaultFeatureTogglesDropRemovedAccelerationKeys(t *testing.T) {
	raw, err := json.Marshal(DefaultFeatureToggles())
	if err != nil {
		t.Fatalf("marshal defaults: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("unmarshal defaults: %v", err)
	}
	for _, key := range []string{"github", "huggingface"} {
		if _, ok := m[key]; ok {
			t.Fatalf("FeatureToggles still exposes %q: %s", key, raw)
		}
	}
	if m["docker_hub"] != true {
		t.Fatalf("docker_hub default lost: %s", raw)
	}
}

func TestLoadFeaturesToleratesLegacyGitHubKeys(t *testing.T) {
	useTestDB(t)
	if err := migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	legacy := `{"docker_hub":true,"github":true,"huggingface":false,"image_search":true,` +
		`"offline_image":true,"public_mirror":false}`
	if _, err := DB.Exec(
		`INSERT INTO settings (key, value, updated_at) VALUES (?, ?, ?)`,
		KeyFeatures, legacy, Now(),
	); err != nil {
		t.Fatalf("insert legacy settings: %v", err)
	}

	got := LoadFeatures()
	if !got.DockerHub || !got.ImageSearch || !got.OfflineImage {
		t.Fatalf("legacy flags lost: %#v", got)
	}
	if got.PublicMirror {
		t.Fatalf("public_mirror should stay false: %#v", got)
	}
}
```

- [ ] **Step 2: 运行测试，确认第一个用例失败**

Run: `cd src && go test ./db/ -run 'TestDefaultFeatureTogglesDropRemovedAccelerationKeys|TestLoadFeaturesToleratesLegacyGitHubKeys' -v`
Expected: `TestDefaultFeatureTogglesDropRemovedAccelerationKeys` FAIL（报 `FeatureToggles still exposes "github"`）。第二个用例应当已经通过。

- [ ] **Step 3: 从 FeatureToggles 结构体删除两个字段**

在 `src/db/settings.go` 中把：

```go
// FeatureToggles controls each acceleration path.
type FeatureToggles struct {
	DockerHub    bool `json:"docker_hub"`
	GitHub       bool `json:"github"`
	HuggingFace  bool `json:"huggingface"`
	ImageSearch  bool `json:"image_search"`
	OfflineImage bool `json:"offline_image"`
```

改为：

```go
// FeatureToggles controls each acceleration path.
type FeatureToggles struct {
	DockerHub    bool `json:"docker_hub"`
	ImageSearch  bool `json:"image_search"`
	OfflineImage bool `json:"offline_image"`
```

- [ ] **Step 4: 从默认值删除对应两项**

在 `src/db/settings.go` 的 `DefaultFeatureToggles()` 中把：

```go
	return FeatureToggles{
		DockerHub:    true,
		GitHub:       true,
		HuggingFace:  true,
		ImageSearch:  true,
		OfflineImage: true,
		PublicMirror: false, // 默认关闭公共镜像，需令牌
	}
```

改为：

```go
	return FeatureToggles{
		DockerHub:    true,
		ImageSearch:  true,
		OfflineImage: true,
		PublicMirror: false, // 默认关闭公共镜像，需令牌
	}
```

- [ ] **Step 5: 从公开配置响应删除对应两项**

在 `src/handlers/auth.go` 的 `AuthPublicConfig` 中，把：

```go
		"features": gin.H{
			"docker_hub":    features.DockerHub,
			"github":        features.GitHub,
			"huggingface":   features.HuggingFace,
			"image_search":  features.ImageSearch,
			"offline_image": features.OfflineImage,
			"public_mirror": features.PublicMirror,
			"require_token": !features.AllowPublicDockerPull(),
		},
```

改为：

```go
		"features": gin.H{
			"docker_hub":    features.DockerHub,
			"image_search":  features.ImageSearch,
			"offline_image": features.OfflineImage,
			"public_mirror": features.PublicMirror,
			"require_token": !features.AllowPublicDockerPull(),
		},
```

- [ ] **Step 6: 删除 CheckGitHubAccess**

在 `src/utils/access_control.go` 中删除 `CheckGitHubAccess` 整个函数（注释行 `// CheckGitHubAccess 检查GitHub仓库访问权限` 及其函数体）。该函数唯一调用方 `content_proxy.go` 已在 Task 1 删除。

- [ ] **Step 7: 删除对应的测试用例**

在 `src/utils/access_control_test.go` 中删除调用 `CheckGitHubAccess` 的三处断言（原文件第 77、80、83 行附近）。若删除后该测试函数体为空，则连同函数一起删除。删除前先阅读该文件确认剩余的 Docker 访问控制断言仍然完整保留。

- [ ] **Step 8: 运行测试，确认全部通过**

Run: `cd src && go test ./...`
Expected: PASS。若 `src/utils/access_control_test.go` 或其它文件仍引用 `CheckGitHubAccess`，编译会失败，据错误信息清理。

- [ ] **Step 9: 提交**

```bash
cd e:/Programming/HTML/hubproxy
git add src/db/settings.go src/db/settings_test.go src/handlers/auth.go src/utils/access_control.go src/utils/access_control_test.go
git commit -m "refactor(settings): drop github/huggingface feature toggles and dead access check"
```

---

### Task 3: OAuth 建号用户强制设置密码

`CreateOAuthUser` 给用户生成随机密码后却把 `must_change_password` 置 0，导致该账号永远无法用用户名密码登录，系统也不会提示设置——这是安全缺口。改为置 1，让新建的 OAuth 用户走一次强制改密。

**Files:**
- Modify: `src/db/oauth.go`
- Test: `src/db/oauth_test.go`（新建）

**Interfaces:**
- Consumes: 无
- Produces: `db.CreateOAuthUser(preferredUsername, email string) (*User, error)` 返回的 `User.MustChangePassword` 恒为 `true`。Task 4 依赖这一点决定 OAuth 回跳目标。

- [ ] **Step 1: 新建 src/db/oauth_test.go，先写失败测试**

```go
package db

import "testing"

func TestCreateOAuthUserRequiresPasswordChange(t *testing.T) {
	useTestDB(t)
	if err := migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	user, err := CreateOAuthUser("octocat", "octo@example.com")
	if err != nil {
		t.Fatalf("create oauth user: %v", err)
	}
	if !user.MustChangePassword {
		t.Fatal("oauth-created user must be forced to set a usable password")
	}
}
```

- [ ] **Step 2: 运行测试，确认失败**

Run: `cd src && go test ./db/ -run TestCreateOAuthUserRequiresPasswordChange -v`
Expected: FAIL，报 `oauth-created user must be forced to set a usable password`。

- [ ] **Step 3: 修改 CreateOAuthUser**

在 `src/db/oauth.go` 中把：

```go
	// oauth-created: no force password change
	_, _ = DB.Exec(`UPDATE users SET must_change_password = 0 WHERE id = ?`, u.ID)
	return GetUserByID(u.ID)
```

改为：

```go
	// oauth 建号时密码是随机生成的，用户并不知道。必须强制其走一次改密，
	// 否则该账号永远无法用用户名密码登录。
	_, _ = DB.Exec(`UPDATE users SET must_change_password = 1 WHERE id = ?`, u.ID)
	return GetUserByID(u.ID)
```

同时把上方那句已经过时的注释：

```go
	// random password (user can set later); oauth users skip must_change if we set must_change=0
```

改为：

```go
	// random password; user is forced to replace it on first sign-in
```

- [ ] **Step 4: 运行测试，确认通过**

Run: `cd src && go test ./db/ -run TestCreateOAuthUserRequiresPasswordChange -v`
Expected: PASS。

- [ ] **Step 5: 提交**

```bash
cd e:/Programming/HTML/hubproxy
git add src/db/oauth.go src/db/oauth_test.go
git commit -m "fix(auth): force oauth-created accounts to set a usable password"
```

---

### Task 4: OAuth 回跳标记新账号 + 前端 hash 解析

后端需要告诉前端「这次是自动建号」，前端需要在落地后提示用户。**这两件事必须同一个任务完成**：后端一旦往 hash 里加 `&welcome=1`，现有的 `main.ts` 会把整段 hash 当 token，登录立刻失效。

**Files:**
- Modify: `src/handlers/oauth.go`
- Modify: `web/src/main.ts`
- Modify: `web/src/admin/AdminLayout.vue`
- Test: `src/handlers/oauth_test.go`（新建）

**Interfaces:**
- Consumes: `db.CreateOAuthUser` 返回的 `MustChangePassword == true`（Task 3）
- Produces: `handlers.buildOAuthRedirect(dest, sessionToken string, created bool) string`——纯函数，返回 `dest + "#oauth_token=<escaped>"`，`created` 为真时追加 `&welcome=1`。前端约定：从 `location.hash` 读取 `oauth_token` 与 `welcome` 两个键。

- [ ] **Step 1: 新建 src/handlers/oauth_test.go，先写失败测试**

```go
package handlers

import "testing"

func TestBuildOAuthRedirectAddsWelcomeOnlyForNewAccounts(t *testing.T) {
	existing := buildOAuthRedirect("/admin", "tok123", false)
	if existing != "/admin#oauth_token=tok123" {
		t.Fatalf("existing-account redirect = %q", existing)
	}

	created := buildOAuthRedirect("/admin/change-password", "tok123", true)
	if created != "/admin/change-password#oauth_token=tok123&welcome=1" {
		t.Fatalf("new-account redirect = %q", created)
	}
}
```

- [ ] **Step 2: 运行测试，确认失败**

Run: `cd src && go test ./handlers/ -run TestBuildOAuthRedirectAddsWelcomeOnlyForNewAccounts -v`
Expected: FAIL，编译错误 `undefined: buildOAuthRedirect`。

- [ ] **Step 3: 在 oauth.go 中抽出 buildOAuthRedirect 并跟踪是否建号**

在 `src/handlers/oauth.go` 中，把 `OAuthCallback` 里这段：

```go
	var user *db.User
	if binding != nil {
		user, err = db.GetUserByID(binding.UserID)
		if err != nil {
			redirectOAuthError(c, "绑定用户不存在")
			return
		}
		_ = db.UpsertOAuthBinding(user.ID, provider, info.Subject, info.Email, info.Name, info.AvatarURL)
	} else {
		if !admin.OAuthRegisterEnabled {
			redirectOAuthError(c, "该第三方账号未绑定，且管理员未开启 OAuth 注册")
			return
		}
		preferred := info.Username
		if preferred == "" {
			preferred = info.Name
		}
		user, err = db.CreateOAuthUser(preferred, info.Email)
		if err != nil {
			redirectOAuthError(c, "创建用户失败: "+err.Error())
			return
		}
		if err := db.UpsertOAuthBinding(user.ID, provider, info.Subject, info.Email, info.Name, info.AvatarURL); err != nil {
			redirectOAuthError(c, err.Error())
			return
		}
	}
```

改为（新增 `created := false`，在自动建号分支里置为 `true`）：

```go
	var user *db.User
	created := false
	if binding != nil {
		user, err = db.GetUserByID(binding.UserID)
		if err != nil {
			redirectOAuthError(c, "绑定用户不存在")
			return
		}
		_ = db.UpsertOAuthBinding(user.ID, provider, info.Subject, info.Email, info.Name, info.AvatarURL)
	} else {
		if !admin.OAuthRegisterEnabled {
			redirectOAuthError(c, "该第三方账号未绑定，且管理员未开启 OAuth 注册")
			return
		}
		preferred := info.Username
		if preferred == "" {
			preferred = info.Name
		}
		user, err = db.CreateOAuthUser(preferred, info.Email)
		if err != nil {
			redirectOAuthError(c, "创建用户失败: "+err.Error())
			return
		}
		if err := db.UpsertOAuthBinding(user.ID, provider, info.Subject, info.Email, info.Name, info.AvatarURL); err != nil {
			redirectOAuthError(c, err.Error())
			return
		}
		created = true
	}
```

再把该函数末尾这段：

```go
	dest := "/admin"
	if user.Role != db.RoleAdmin {
		dest = "/admin/user"
	}
	if user.MustChangePassword {
		dest = "/admin/change-password"
	}
	c.Redirect(http.StatusFound, dest+"#oauth_token="+url.QueryEscape(sessionToken))
```

改为：

```go
	dest := "/admin"
	if user.Role != db.RoleAdmin {
		dest = "/admin/user"
	}
	if user.MustChangePassword {
		dest = "/admin/change-password"
	}
	c.Redirect(http.StatusFound, buildOAuthRedirect(dest, sessionToken, created))
```

最后在同文件的 `redirectOAuthError` 上方新增纯函数：

```go
// buildOAuthRedirect 构造登录后的回跳地址，会话令牌放在 URL fragment 中。
// fragment 不会发送给服务端，因此令牌不会进入访问日志或 Referer 头。
// created 为真表示本次是第三方首次登录、系统自动建号，前端据此提示用户。
func buildOAuthRedirect(dest, sessionToken string, created bool) string {
	frag := "oauth_token=" + url.QueryEscape(sessionToken)
	if created {
		frag += "&welcome=1"
	}
	return dest + "#" + frag
}
```

- [ ] **Step 4: 运行测试，确认通过**

Run: `cd src && go test ./handlers/ -run TestBuildOAuthRedirectAddsWelcomeOnlyForNewAccounts -v`
Expected: PASS。

- [ ] **Step 5: 改写 main.ts 的 hash 解析**

后端现在会发出 `#oauth_token=<t>&welcome=1`，而现有代码把整段 hash 当 token。把 `web/src/main.ts` 中：

```ts
// OAuth callback may redirect with #oauth_token=
if (typeof window !== 'undefined' && window.location.hash.startsWith('#oauth_token=')) {
  const token = decodeURIComponent(window.location.hash.slice('#oauth_token='.length))
  if (token) {
    setToken(token)
    history.replaceState(null, '', window.location.pathname + window.location.search)
  }
}
```

改为：

```ts
// OAuth callback may redirect with #oauth_token=<token>&welcome=1
if (typeof window !== 'undefined' && window.location.hash.startsWith('#oauth_token=')) {
  const params = new URLSearchParams(window.location.hash.slice(1))
  const token = params.get('oauth_token') || ''
  if (token) {
    setToken(token)
  }
  // 第三方首次登录时后端自动建号，落地后由 AdminLayout 弹一次提示
  if (params.get('welcome') === '1') {
    sessionStorage.setItem('hubproxy_oauth_welcome', '1')
  }
  history.replaceState(null, '', window.location.pathname + window.location.search)
}
```

- [ ] **Step 6: 在 AdminLayout 里消费 welcome 标记并弹提示**

`web/src/admin/AdminLayout.vue` 目前**没有** `onMounted`，也没有引入 toast。先补 import——把：

```ts
import { computed, ref } from 'vue'
```

改为：

```ts
import { computed, onMounted, ref } from 'vue'
```

在 `import { site } from '@/lib/site'` 下一行补上：

```ts
import { toastSuccess } from '@/lib/toast'
```

然后在 `const { user, logout, isAdmin } = useAuth()` 之后、`const open = ref(false)` 之前，插入：

```ts
// 第三方首次登录时后端自动建号，落地后提示一次
onMounted(() => {
  if (sessionStorage.getItem('hubproxy_oauth_welcome') !== '1') return
  sessionStorage.removeItem('hubproxy_oauth_welcome')
  const name = user.value?.username || ''
  toastSuccess(
    name
      ? `已自动为你创建账号：${name}，请先设置密码`
      : '已自动为你创建账号，请先设置密码',
  )
})
```

> 说明：新建的 OAuth 用户 `must_change_password` 为真（Task 3），router guard 会把用户导向 `/admin/change-password`，而该页同样包在 `AdminLayout` 内，因此提示一定会在用户看到的第一个控制台页面上出现。

- [ ] **Step 7: 运行测试与前端构建**

Run: `cd src && go test ./... && cd ../web && npm run build`
Expected: Go 测试全绿；`vue-tsc` 无类型错误、构建成功。

- [ ] **Step 8: 提交**

```bash
cd e:/Programming/HTML/hubproxy
git add src/handlers/oauth.go src/handlers/oauth_test.go web/src/main.ts web/src/admin/AdminLayout.vue
git commit -m "feat(auth): flag oauth auto-registration in redirect and prompt the user"
```

---

### Task 5: 移除 GitHub / Hugging Face 的前端显示

删掉首页的链接加速器、后台的功能开关与分类筛选项，以及 `admin/api.ts` 里依赖 GitHub 代理的两处逻辑。

**Files:**
- Modify: `web/src/pages/HomePage.vue`
- Modify: `web/src/admin/pages/FeaturesPage.vue`
- Modify: `web/src/admin/pages/PullsPage.vue`
- Modify: `web/src/admin/pages/ImagesPage.vue`
- Modify: `web/src/admin/api.ts`
- Modify: `web/src/lib/site.ts`
- Modify: `src/handlers/user_console.go`

**Interfaces:**
- Consumes: `AuthPublicConfig` 不再返回 `features.github` / `features.huggingface`（Task 2）
- Produces: `web/src/admin/api.ts` 的 `FeatureToggles` 接口只剩 `docker_hub` / `image_search` / `offline_image` / `public_mirror`；`categoryLabel` / `pullSourceLabel` / `displayPullName` 不再有 github / huggingface 分支。

- [ ] **Step 1: 精简 HomePage.vue 的脚本段**

在 `web/src/pages/HomePage.vue` 中，把 import 块：

```ts
import {
  Check,
  Clipboard,
  Container,
  Globe2,
  Link2,
  Rocket,
  Search,
  Shield,
  Sparkles,
  Zap,
} from 'lucide-vue-next'
import Button from '@/components/ui/Button.vue'
import Input from '@/components/ui/Input.vue'
import PageHero from '@/components/PageHero.vue'
import { copyText } from '@/lib/utils'
import { site } from '@/lib/site'
import { adminApi, getToken } from '@/admin/api'
import { setAccessToken } from '@/lib/accessToken'
```

改为：

```ts
import {
  Container,
  Globe2,
  Search,
  Shield,
  Zap,
} from 'lucide-vue-next'
import Button from '@/components/ui/Button.vue'
import PageHero from '@/components/PageHero.vue'
import { site } from '@/lib/site'
import { adminApi, getToken } from '@/admin/api'
import { setAccessToken } from '@/lib/accessToken'
```

把状态声明块：

```ts
const input = ref('')
const output = ref('')
const error = ref('')
const copied = ref(false)
const accessToken = ref('')
const authenticated = ref(false)
const runtimeFeatures = ref({
  docker_hub: true,
  github: true,
  huggingface: true,
  image_search: true,
  offline_image: true,
  public_mirror: false,
})
```

改为：

```ts
const accessToken = ref('')
const authenticated = ref(false)
const runtimeFeatures = ref({
  docker_hub: true,
  image_search: true,
  public_mirror: false,
})
```

把 `features` 数组：

```ts
const features = [
  { icon: Rocket, label: 'GitHub 加速' },
  { icon: Container, label: 'Docker 镜像' },
  { icon: Sparkles, label: 'Hugging Face' },
] as const
```

改为：

```ts
const features = [
  { icon: Container, label: 'Docker 镜像' },
  { icon: Zap, label: '多源 Registry' },
] as const
```

把 `highlights` 的第一条文案：

```ts
  {
    icon: Zap,
    title: '多源统一加速',
    desc: 'Docker、GitHub、Hugging Face 一站接入，减少切换成本。',
  },
```

改为：

```ts
  {
    icon: Zap,
    title: '多源统一加速',
    desc: 'Docker Hub、GHCR、GCR、Quay 等 Registry 一站接入，减少切换成本。',
  },
```

**删除**从 `const githubSources = [` 到 `const dockerExamples = computed` 之前的整段（即 `githubSources` 与 `visibleGithubSources`）。

**删除**从 `const githubPatterns = [` 一直到 `onMounted` 之前的整段，即：`githubPatterns`、`huggingFacePatterns`、`sourceKind`、`contentPrefix`、`contentUsageExample`、`formatLink`、`onCopy`、`onOpen`。`dockerExamples` 与 `requireToken` 必须保留。

- [ ] **Step 2: 精简 HomePage.vue 的模板段**

把 hero 的副标题：

```html
      :subtitle="`Docker · GitHub · Hugging Face 多源加速 · ${site.tagline}`"
```

改为：

```html
      :subtitle="`Docker 多源镜像加速 · ${site.tagline}`"
```

把 hero 的按钮组：

```html
      <div class="flex flex-wrap justify-center gap-2 pt-4">
        <a href="#accelerate">
          <Button>立即加速</Button>
        </a>
```

改为：

```html
      <div class="flex flex-wrap justify-center gap-2 pt-4">
        <a href="#docker-pull">
          <Button>立即拉取</Button>
        </a>
```

**删除**整个链接加速 section，即从 `<section id="accelerate" class="surface-panel field-block">` 到它对应的 `</section>`（包含标题「链接加速」、输入框、error/output 的 Transition 块）。

在「支持的镜像源」section 中，把说明段落：

```html
          <p v-if="contentUsageExample" class="mt-3 text-xs text-muted-foreground">
```

改为（`contentUsageExample` 已删除，但这段 Docker 用法说明需要保留，仅去掉条件）：

```html
          <p class="mt-3 text-xs text-muted-foreground">
```

**删除**整个 GitHub / Hugging Face 卡片，即从 `<div class="surface-panel rounded-xl border border-border/60 p-5 sm:col-span-2">` 中标题为 `GitHub / Hugging Face` 的那一个 div 开始，到它对应的 `</div>` 结束（含 `v-for="item in visibleGithubSources"` 与 `contentUsageExample` 用法段落）。注意只删这一个卡片，保留同级的 Docker Registry 卡片与它们共同的外层 `<div class="grid gap-3 sm:grid-cols-2">`。

把「支持的镜像源」标题下的副标题：

```html
        <p class="text-muted-foreground">
          当前程序内置并启用的 Registry 与文件加速源
        </p>
```

改为：

```html
        <p class="text-muted-foreground">
          当前程序内置并启用的 Registry 加速源
        </p>
```

把「Docker 镜像加速」的 terminal section 开头：

```html
    <section class="space-y-6 pt-12">
      <div class="space-y-1 text-center">
        <h2 class="text-sm font-semibold tracking-[0.16em] text-muted-foreground uppercase">
          Docker 镜像加速
        </h2>
```

改为（补上 hero 按钮指向的锚点 id）：

```html
    <section id="docker-pull" class="space-y-6 pt-12">
      <div class="space-y-1 text-center">
        <h2 class="text-sm font-semibold tracking-[0.16em] text-muted-foreground uppercase">
          Docker 镜像加速
        </h2>
```

- [ ] **Step 3: 检查 HomePage 自身已清理干净**

Run: `cd web && npm run build`

这一步预期**会通过**，但通过并不代表 HomePage 删干净了——`HomePage.vue` 的 `runtimeFeatures` 是本地推断的对象字面量，与 `FeatureToggles` 接口没有结构关联，而 TS 接口的 github/huggingface 字段要到 Step 6 才移除，所以此刻没有任何类型检查会因 HomePage 的删除而失败。

因此这一步只确认一件事：**报错信息里没有一条来自 `HomePage.vue`**。若出现 `Cannot find name 'formatLink'`、`Cannot find name 'contentUsageExample'` 之类指向 HomePage 的错误，说明 Step 1 / Step 2 删漏了，回去补齐后再继续。清理是否彻底，以 Step 9 的构建和 Task 8 的终检 grep 为准。

- [ ] **Step 4: 删除后台的功能开关**

在 `web/src/admin/pages/FeaturesPage.vue` 中，`<CardContent class="space-y-3">` 内依次排列着六个开关卡片。精确删除中间这两个：

```html
        <div class="flex items-center justify-between rounded-lg border border-border p-4">
          <div>
            <div class="font-medium">GitHub 加速</div>
            <div class="text-sm text-muted-foreground">Release / Raw / Clone / API</div>
          </div>
          <Switch v-model:checked="settings.features.github" />
        </div>
        <div class="flex items-center justify-between rounded-lg border border-border p-4">
          <div>
            <div class="font-medium">Hugging Face</div>
            <div class="text-sm text-muted-foreground">模型与 LFS 文件</div>
          </div>
          <Switch v-model:checked="settings.features.huggingface" />
        </div>
```

删除后 `CardContent` 内应剩四个卡片，顺序为：Docker Hub、镜像搜索、离线镜像包、开启公共镜像。「离线镜像包」开关**保留**——后端打包能力仍在，只是前端不再有入口。如果 `<script setup>` 段还有对本页已不再使用的类型引用，一并清理。

- [ ] **Step 5: 删除后台的分类筛选项**

在 `web/src/admin/pages/PullsPage.vue` 与 `web/src/admin/pages/ImagesPage.vue` 中，各自删除分类筛选下拉里的这两项：

```ts
  { value: 'github', label: 'GitHub' },
  { value: 'huggingface', label: 'Hugging Face' },
```

- [ ] **Step 6: 清理 admin/api.ts**

在 `web/src/admin/api.ts` 中，把 `FeatureToggles` 接口：

```ts
export interface FeatureToggles {
  docker_hub: boolean
  github: boolean
  huggingface: boolean
  image_search: boolean
  offline_image: boolean
  public_mirror: boolean
}
```

改为：

```ts
export interface FeatureToggles {
  docker_hub: boolean
  image_search: boolean
  offline_image: boolean
  public_mirror: boolean
}
```

删除 `request()` 中的 `STALE_SERVER` 启发式判断。这段是：

```ts
    // 旧进程未注册路由时会落到 GitHub 代理，返回纯文本「无效输入」
    if (res.status === 403 && (msg.includes('无效输入') || text.includes('无效输入'))) {
      throw new ApiError(
        '接口未找到（无效输入）。请停掉旧 hubproxy 进程后用最新代码重启：cd src; go run .',
        res.status,
        'STALE_SERVER',
      )
    }
    throw new ApiError(msg, res.status, data?.code)
```

改为：

```ts
    throw new ApiError(msg, res.status, data?.code)
```

把三个标签函数：

```ts
export function categoryLabel(category?: string): string {
  if (category === 'github') return 'GitHub'
  if (category === 'huggingface') return 'Hugging Face'
  return category || '-'
}

export function pullSourceLabel(p: Pick<PullSession, 'category' | 'registry'>): string {
  if (p.category === 'github') return 'GitHub'
  if (p.category === 'huggingface') return 'Hugging Face'
  return p.registry || 'Docker'
}

export function displayPullName(p: PullSession): string {
  if (p.category === 'github' || p.category === 'huggingface') {
    const clean = (p.image_name || '').split('?')[0].replace(/\/$/, '')
    const base = clean.split('/').filter(Boolean).pop()
    return base || p.tag || p.image_name
  }
  return p.tag ? `${p.image_name}:${p.tag}` : p.image_name
}
```

改为：

```ts
export function categoryLabel(category?: string): string {
  return category || '-'
}

export function pullSourceLabel(p: Pick<PullSession, 'category' | 'registry'>): string {
  return p.registry || 'Docker'
}

export function displayPullName(p: PullSession): string {
  return p.tag ? `${p.image_name}:${p.tag}` : p.image_name
}
```

- [ ] **Step 7: 更新站点描述**

在 `web/src/lib/site.ts` 中把 description 一行：

```ts
  description: '多源镜像加速服务，支持 Docker、GitHub、Hugging Face。',
```

改为：

```ts
  description: '多源镜像加速服务，支持 Docker Hub、GHCR、GCR、Quay 等 Registry。',
```

- [ ] **Step 8: 清除用户控制台里仍指向已删除路由的示例**

`src/handlers/user_console.go` 仍在向用户控制台下发 `/令牌/gh/...` 与 `/令牌/hf/...` 形式的示例地址。这些路由已在 Task 1 删除，现在会 404——用户照着复制粘贴必然失败。

**这一步只改后端，前端零改动**：`web/src/admin/api.ts` 把 `notes` 声明为 `string[]`、`examples` 声明为 `Record<string, string>`，`UserTokenPage.vue` 对两者都是泛型遍历渲染，所以后端少发几条，前端自然少显示几条。

删除 `notes` 数组中的这两行：

```go
			"GitHub 文件加速：" + "https://" + host + "/令牌/gh/github.com/owner/repo/releases/...",
			"Hugging Face 文件加速：" + "https://" + host + "/令牌/hf/huggingface.co/user/repo/resolve/...",
```

删除 `buildTokenExamples` 返回值中的这两个键：

```go
		"github":       "https://" + host + "/" + token + "/gh/github.com/owner/repo/releases/download/v1.0.0/app.tar.gz",
		"huggingface":  "https://" + host + "/" + token + "/hf/huggingface.co/user/model/resolve/main/config.json",
```

删除后 `notes` 剩 7 条、`examples` 剩 7 条（原 9 条减 2），全部是 Docker 相关。保留 `ghcr` 与 `gitlab` 两个示例——那是 Docker Registry，与本次移除的 GitHub 文件加速无关。

自查：

```bash
cd e:/Programming/HTML/hubproxy
grep -rni "github\|huggingface" src/handlers/user_console.go | grep -v "gin-gonic"
```

Expected: 无输出。

> `| grep -v "gin-gonic"` 是必需的：该文件导入了 `github.com/gin-gonic/gin`，它必然匹配 `github`，属于误报。踩过一次。

- [ ] **Step 9: 构建与测试验证**

Run: `cd src && go test ./... && cd ../web && npm run build`

Expected：Go 侧相对基线**无新增失败**（基线的 8 个既有失败见 Global Constraints，它们本来就红，不是本任务引入的）；前端 `vue-tsc` 无类型错误。若前端报 `Property 'github' does not exist on type 'FeatureToggles'` 之类，说明还有页面在引用已删除字段，据错误信息清理。

- [ ] **Step 10: 提交**

```bash
cd e:/Programming/HTML/hubproxy
git add web/src/pages/HomePage.vue web/src/admin/pages/FeaturesPage.vue web/src/admin/pages/PullsPage.vue \
        web/src/admin/pages/ImagesPage.vue web/src/admin/api.ts web/src/lib/site.ts \
        src/handlers/user_console.go
git commit -m "refactor: remove github/huggingface surfaces from site and console"
```

---

### Task 6: 移除离线镜像前端入口

离线打包后端 API 保留，只删前端页面、路由、导航入口与随之成为死代码的 API 封装。已验证 `/api/image/*` 与 `docker pull` 的 `/v2/*` 链路完全独立。

**Files:**
- Delete: `web/src/pages/ImagesPage.vue`
- Modify: `web/src/api.ts`
- Modify: `web/src/router/index.ts`
- Modify: `web/src/components/AppShell.vue`
- Modify: `src/main.go`

**Interfaces:**
- Consumes: 无
- Produces: `web/src/api.ts` 只保留搜索相关导出：`Repository`、`SearchResponse`、`TagInfo`、`TagPageResult`、`searchImages`、`fetchTags`。后端 `registerFrontendRoutes` 不再注册 `/images`。

- [ ] **Step 1: 删除离镜像页面与死代码**

```bash
cd e:/Programming/HTML/hubproxy
git rm web/src/pages/ImagesPage.vue
```

在 `web/src/api.ts` 中删除**仅**被该页使用的导出：`PrepareDownloadResponse`、`ImageInfoResponse`、`prepareSingleDownload`、`fetchImageInfo`、`prepareBatchDownload`、`triggerDownload`。**保留** `Repository`、`SearchResponse`、`TagInfo`、`TagPageResult`、`searchImages`、`fetchTags`——`SearchPage.vue` 仍在用。

- [ ] **Step 2: 从 vue-router 移除 /images**

在 `web/src/router/index.ts` 中删除整个 `/images` 路由对象：

```ts
    {
      path: '/images',
      component: ImagesPage,
      meta: { title: '离线镜像下载' },
    },
```

并删除文件顶部对应的 `import ImagesPage from '@/pages/ImagesPage.vue'`。

- [ ] **Step 3: 从导航移除离线镜像入口**

在 `web/src/components/AppShell.vue` 的 `links` 数组中删除这一项：

```ts
  { to: '/images', label: '离线镜像', icon: Container },
```

并从 `lucide-vue-next` 的 import 中移除已不再使用的 `Container`。

- [ ] **Step 4: 从后端 SPA 路由移除 /images**

在 `src/main.go` 的 `registerFrontendRoutes` 中，删除前端禁用分支里的：

```go
		router.GET("/images", notFound)
```

与启用分支里的：

```go
	router.GET("/images", serveSPA)
```

删除后 `/images` 会落到 `NoRoute` 返回 404 JSON，不会再返回一个没有对应前端路由的空 SPA。

- [ ] **Step 5: 运行测试与构建**

Run: `cd src && go test ./... && cd ../web && npm run build`
Expected: Go 测试全绿（`TestFrontendDisabledRoutesReturnNotFound` 仍会通过——前端禁用时 `/images` 由 `NoRoute` 返回 404）；前端构建通过。

- [ ] **Step 6: 提交**

```bash
cd e:/Programming/HTML/hubproxy
git add web/src/api.ts web/src/router/index.ts web/src/components/AppShell.vue src/main.go
git commit -m "refactor(web): retire the offline image download page"
```

（`web/src/pages/ImagesPage.vue` 的删除已在 Step 1 由 `git rm` 暂存。**不要用 `git add -A`**——那会把未跟踪的 `docs/superpowers/` 设计文档卷进这个代码提交，也会掩盖漏改的文件。）

---

### Task 7: 认证路由重构与登录态导航

用一个 `AuthPage.vue` 同时承载 `/login` 与 `/register`，删掉 `/admin/login`，并让顶部导航依据登录态切换。

**Files:**
- Create: `web/src/pages/AuthPage.vue`
- Delete: `web/src/admin/pages/LoginPage.vue`
- Modify: `web/src/router/index.ts`
- Modify: `web/src/App.vue`
- Modify: `web/src/components/AppShell.vue`
- Modify: `web/src/admin/AdminLayout.vue`
- Modify: `src/main.go`
- Modify: `src/handlers/oauth.go`
- Test: `src/main_test.go`
- Test: `src/handlers/oauth_test.go`

**Interfaces:**
- Consumes: `useAuth()`（`web/src/admin/auth.ts`）导出的 `{ user, loaded, isAuthed, isAdmin, bootstrap, login, logout, setUser }`；`adminApi.publicConfig()` 返回的 `form_register_enabled` / `email_register_enabled` / `oauth_login_enabled` / `oauth.display_name` / `site`
- Produces: 路由 `/login` 与 `/register` 指向 `AuthPage`，各自通过 `props: { mode: 'login' | 'register' }` 区分；`AppShell` 暴露 `isActive(to: string): boolean` 与计算属性 `navLinks: NavLink[]`。

- [ ] **Step 1: 新建 web/src/pages/AuthPage.vue**

```vue
<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { ArrowLeft, Container, Loader2, ShieldCheck } from 'lucide-vue-next'
import Button from '@/components/ui/Button.vue'
import Input from '@/components/ui/Input.vue'
import Label from '@/components/ui/Label.vue'
import Card from '@/components/ui/Card.vue'
import CardContent from '@/components/ui/CardContent.vue'
import CardHeader from '@/components/ui/CardHeader.vue'
import CardTitle from '@/components/ui/CardTitle.vue'
import { adminApi } from '@/admin/api'
import { useAuth } from '@/admin/auth'
import { applySiteFromApi, site } from '@/lib/site'
import { toastError, toastSuccess } from '@/lib/toast'

const props = defineProps<{ mode: 'login' | 'register' }>()

const router = useRouter()
const route = useRoute()
const { login } = useAuth()
const username = ref('')
const password = ref('')
const email = ref('')
const code = ref('')
const error = ref('')
const loading = ref(false)
const sending = ref(false)
const registerEnabled = ref(false)
const emailRegister = ref(false)
const oauthLogin = ref(false)
const oauthLabel = ref('OAuth2 登录')

const isRegister = computed(() => props.mode === 'register')

onMounted(async () => {
  const qErr = route.query.oauth_error
  if (typeof qErr === 'string' && qErr) {
    error.value = qErr
    toastError(qErr)
  }
  try {
    const cfg = await adminApi.publicConfig()
    registerEnabled.value = !!(cfg.form_register_enabled ?? cfg.register_enabled)
    emailRegister.value = !!cfg.email_register_enabled
    oauthLogin.value = !!(cfg.oauth_login_enabled && cfg.oauth?.enabled)
    if (cfg.oauth?.display_name) oauthLabel.value = cfg.oauth.display_name
    if (cfg.site) applySiteFromApi(cfg.site)
  } catch {
    /* ignore */
  }
})

function startOAuth() {
  window.location.href = '/api/admin/oauth/start?mode=login'
}

async function sendCode() {
  if (!email.value.trim()) {
    toastError('请先填写邮箱')
    return
  }
  sending.value = true
  try {
    const res = await adminApi.sendRegisterCode(email.value.trim())
    toastSuccess(res.message || '验证码已发送')
  } catch (e: any) {
    toastError(e?.message || '发送失败')
  } finally {
    sending.value = false
  }
}

async function submit() {
  error.value = ''
  loading.value = true
  try {
    if (isRegister.value) {
      await adminApi.register(
        username.value,
        password.value,
        emailRegister.value ? email.value : undefined,
        emailRegister.value ? code.value : undefined,
      )
      toastSuccess('注册成功，请登录')
      await router.replace('/login')
      return
    }
    const user = await login(username.value, password.value)
    if (user.must_change_password) {
      await router.replace('/admin/change-password')
    } else {
      await router.replace(user.role === 'admin' ? '/admin' : '/admin/user')
    }
  } catch (e: any) {
    error.value = e?.message || (isRegister.value ? '注册失败' : '登录失败')
    toastError(error.value)
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="relative flex min-h-screen flex-col overflow-hidden">
    <div class="pointer-events-none absolute inset-0 -z-10">
      <div class="absolute inset-0 bg-background" />
      <div class="absolute -left-24 top-0 size-[28rem] rounded-full bg-primary/10 blur-3xl" />
      <div class="absolute -right-20 bottom-0 size-[24rem] rounded-full bg-primary/8 blur-3xl" />
    </div>

    <header class="flex items-center justify-between px-5 py-4 sm:px-8">
      <RouterLink
        to="/"
        class="inline-flex items-center gap-2 rounded-full border border-border/70 bg-background/70 px-3.5 py-2 text-sm text-muted-foreground backdrop-blur-md transition-colors hover:border-border hover:text-foreground"
      >
        <ArrowLeft class="size-4" />
        返回主页
      </RouterLink>
      <div class="hidden items-center gap-2 text-xs text-muted-foreground sm:flex">
        <ShieldCheck class="size-3.5 text-primary" />
        {{ site.fullName || site.name }}
      </div>
    </header>

    <div class="flex flex-1 items-center justify-center px-4 pb-12">
      <Card class="w-full max-w-md border-border/70 bg-background/80 shadow-xl backdrop-blur-xl">
        <CardHeader class="items-center space-y-3 pb-2 text-center">
          <div class="flex size-14 items-center justify-center rounded-2xl bg-primary text-primary-foreground shadow-lg shadow-primary/25">
            <Container class="size-6" />
          </div>
          <div class="space-y-1.5">
            <CardTitle class="font-display text-2xl tracking-tight">{{ site.name }}</CardTitle>
            <p class="text-sm text-muted-foreground">
              {{ isRegister ? '创建账号' : '登录控制台' }}
            </p>
          </div>
        </CardHeader>
        <CardContent class="pt-2">
          <p
            v-if="isRegister && !registerEnabled"
            class="rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive"
          >
            当前站点已关闭注册，请联系管理员开通账号。
          </p>
          <form v-else class="space-y-4" @submit.prevent="submit">
            <div class="space-y-2">
              <Label for="username">用户名</Label>
              <Input id="username" v-model="username" autocomplete="username" placeholder="请输入用户名" required />
            </div>
            <div class="space-y-2">
              <Label for="password">密码</Label>
              <Input
                id="password"
                v-model="password"
                type="password"
                :autocomplete="isRegister ? 'new-password' : 'current-password'"
                :placeholder="isRegister ? '至少 8 位' : '请输入密码'"
                required
              />
            </div>
            <template v-if="isRegister && emailRegister">
              <div class="space-y-2">
                <Label>邮箱</Label>
                <Input v-model="email" type="email" placeholder="用于接收验证码" required />
              </div>
              <div class="space-y-2">
                <Label>验证码</Label>
                <div class="flex gap-2">
                  <Input v-model="code" placeholder="邮箱验证码" required />
                  <Button type="button" variant="outline" class="shrink-0 rounded-xl" :disabled="sending" @click="sendCode">
                    {{ sending ? '发送中…' : '获取验证码' }}
                  </Button>
                </div>
              </div>
            </template>
            <p v-if="error" class="rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">{{ error }}</p>
            <Button class="h-11 w-full rounded-xl" :disabled="loading" type="submit">
              <Loader2 v-if="loading" class="size-4 animate-spin" />
              {{ isRegister ? '注册' : '登录' }}
            </Button>

            <template v-if="oauthLogin && !isRegister">
              <div class="relative py-1 text-center text-xs text-muted-foreground">
                <span class="relative z-10 bg-background/80 px-2">或</span>
                <div class="absolute inset-x-0 top-1/2 h-px bg-border" />
              </div>
              <Button type="button" variant="outline" class="h-11 w-full rounded-xl" @click="startOAuth">
                {{ oauthLabel }}
              </Button>
            </template>
          </form>

          <p class="pt-4 text-center text-sm text-muted-foreground">
            <template v-if="isRegister">
              已有账号？<RouterLink to="/login" class="text-primary hover:underline">去登录</RouterLink>
            </template>
            <template v-else-if="registerEnabled">
              没有账号？<RouterLink to="/register" class="text-primary hover:underline">注册账号</RouterLink>
            </template>
          </p>
        </CardContent>
      </Card>
    </div>
  </div>
</template>
```

- [ ] **Step 2: 删除旧的登录页**

```bash
cd e:/Programming/HTML/hubproxy
git rm web/src/admin/pages/LoginPage.vue
```

- [ ] **Step 3: 重写路由表**

在 `web/src/router/index.ts` 中，把顶部 import 块里的：

```ts
import LoginPage from '@/admin/pages/LoginPage.vue'
```

改为：

```ts
import AuthPage from '@/pages/AuthPage.vue'
```

把路由表中的 `/admin/login` 对象：

```ts
    {
      path: '/admin/login',
      component: LoginPage,
      meta: { title: '登录', public: true },
    },
```

替换为：

```ts
    {
      path: '/login',
      component: AuthPage,
      props: { mode: 'login' },
      meta: { title: '登录' },
    },
    {
      path: '/register',
      component: AuthPage,
      props: { mode: 'register' },
      meta: { title: '注册' },
    },
```

- [ ] **Step 3b: 让全屏页不再套站点外壳**

> **这一步是本计划最初遗漏的。** 审查阶段才发现：`web/src/App.vue:9` 用 `route.path.startsWith('/admin')` 决定是否套 `AppShell`。旧的登录页在 `/admin/login`，落在该前缀内，所以是裸渲染的；路由迁到 `/login` 后就跑到前缀外，`AuthPage` 自身的 `min-h-screen` 全屏布局会被 `AppShell` 的页头、`<main>`、页脚再包一层——双份页头、登录页导航里显示「登录账号」、全屏卡片下压着页脚。

不要用「再补一份路径清单」的方式修。本次改造的教训恰恰是**一处清单会与实际路由在另一处漂移**（`App.vue` 就是这样被漏掉的）。把判定放到路由定义旁边，用 route meta 标记。

**(a)** 在 `web/src/router/index.ts` 中，给 `/login`、`/register` 两个路由的 meta 加上 `bare: true`：

```ts
    {
      path: '/login',
      component: AuthPage,
      props: { mode: 'login' },
      meta: { title: '登录', bare: true },
    },
    {
      path: '/register',
      component: AuthPage,
      props: { mode: 'register' },
      meta: { title: '注册', bare: true },
    },
```

再给 `/admin` 父路由的 meta 加上 `bare: true`：

```ts
    {
      path: '/admin',
      component: AdminLayout,
      meta: { requiresAuth: true, bare: true },
```

**只在父记录上加一次，不要在每个子路由重复。** vue-router 会把所有匹配记录的 `meta` 合并，`/admin/*` 的子路由自动继承。

**(b)** 在 `web/src/App.vue` 中，把：

```ts
const route = useRoute()
const isAdmin = computed(() => route.path.startsWith('/admin'))
```

改为：

```ts
const route = useRoute()
// 全屏独立页面（控制台、登录、注册）不套站点外壳，直接整屏渲染
const isBare = computed(() => route.meta.bare === true)
```

并把模板里的两处 `isAdmin` 改为 `isBare`：

```html
  <RouterView v-if="isBare" />
  <AppShell v-else>
```

**验证**：`/login`、`/register`、`/admin` 及其子路由均为裸渲染；`/` 与 `/search` 仍在 `AppShell` 内。特别要确认 `/admin` 的子路由确实是**继承**而非需要重复声明——若继承不成立，控制台会反过来被套进站点外壳，那是方向相反的同一个 bug。

- [ ] **Step 4: 重写路由守卫**

在 `web/src/router/index.ts` 中，把整个 `router.beforeEach(...)` 块替换为：

```ts
const AUTH_ENTRY_PATHS = ['/login', '/register']

function consoleHome(user: User): string {
  if (user.must_change_password) return '/admin/change-password'
  return user.role === 'admin' ? '/admin' : '/admin/user'
}

router.beforeEach(async (to) => {
  const isAuthEntry = AUTH_ENTRY_PATHS.includes(to.path)
  if (!isAuthEntry && !to.path.startsWith('/admin')) return true

  const auth = useAuth()
  if (!auth.loaded.value) {
    await auth.bootstrap()
  }

  // 已登录用户不应再看到登录页/注册页
  if (isAuthEntry) {
    if (auth.user.value) return consoleHome(auth.user.value)
    return true
  }

  if (!auth.user.value) {
    return { path: '/login', query: { redirect: to.fullPath } }
  }

  if (auth.user.value.must_change_password && to.path !== '/admin/change-password') {
    return '/admin/change-password'
  }

  if (to.meta.requiresAdmin && auth.user.value.role !== 'admin') {
    return '/admin/user'
  }

  // 普通用户落到 /admin 根路径
  if (to.path === '/admin' && auth.user.value.role !== 'admin') {
    return '/admin/user'
  }

  return true
})
```

同时把 import 块中的：

```ts
import { getToken } from '@/admin/api'
import { useAuth } from '@/admin/auth'
```

改为：

```ts
import type { User } from '@/admin/api'
import { useAuth } from '@/admin/auth'
```

（`getToken` 在新守卫里不再使用；`User` 类型是 `consoleHome` 的形参所需。）

- [ ] **Step 5: 重写 AppShell 的导航逻辑**

在 `web/src/components/AppShell.vue` 中，把 import 块（**注意：这是 Task 6 处理过之后的状态，`Container` 已在 Task 6 移除**）：

```ts
import { computed, onMounted, ref } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import { ExternalLink, Github, Menu, Rocket, Search, X, Zap } from 'lucide-vue-next'
import Button from '@/components/ui/Button.vue'
import ThemeToggle from '@/components/ThemeToggle.vue'
import { site } from '@/lib/site'
```

改为：

```ts
import { computed, onMounted, ref, type Component } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import { ExternalLink, Github, LogIn, Menu, Rocket, Search, UserPlus, X, Zap } from 'lucide-vue-next'
import Button from '@/components/ui/Button.vue'
import ThemeToggle from '@/components/ThemeToggle.vue'
import { site } from '@/lib/site'
import { adminApi } from '@/admin/api'
import { useAuth } from '@/admin/auth'

type NavLink = { to: string; label: string; icon: Component }
```

把静态的 `links` 常量（同样是 Task 6 处理后的三元素版本）：

```ts
const links = [
  { to: '/', label: '镜像加速', icon: Rocket },
  { to: '/search', label: '镜像搜索', icon: Search },
  { to: '/admin', label: '管理后台', icon: Zap },
] as const

const currentPath = computed(() => route.path)
```

替换为：

```ts
const { user, bootstrap } = useAuth()
const registerEnabled = ref(false)

const navLinks = computed<NavLink[]>(() => {
  const links: NavLink[] = [
    { to: '/', label: '镜像加速', icon: Rocket },
    { to: '/search', label: '镜像搜索', icon: Search },
  ]
  if (user.value) {
    links.push({ to: '/admin', label: '管理后台', icon: Zap })
    return links
  }
  links.push({ to: '/login', label: '登录账号', icon: LogIn })
  // 注册关闭时不给入口，避免点进一个必然被拒的页面
  if (registerEnabled.value) {
    links.push({ to: '/register', label: '注册账号', icon: UserPlus })
  }
  return links
})

function isActive(to: string): boolean {
  if (to === '/admin') return route.path.startsWith('/admin')
  return route.path === to
}
```

把 `onMounted` 回调改为（保留原有主题初始化，追加登录态与注册开关拉取）：

```ts
onMounted(async () => {
  const saved = localStorage.getItem(STORAGE_KEY)
  if (saved === 'dark' || saved === 'light') {
    applyTheme(saved === 'dark')
  } else {
    applyTheme(window.matchMedia('(prefers-color-scheme: dark)').matches)
  }

  await bootstrap()
  try {
    const cfg = await adminApi.publicConfig()
    registerEnabled.value = !!(cfg.form_register_enabled ?? cfg.register_enabled)
  } catch {
    /* 拉取失败时按未开启注册处理 */
  }
})
```

- [ ] **Step 6: 更新 AppShell 模板**

在桌面导航中把：

```html
        <nav class="hidden items-center gap-1.5 md:flex">
          <RouterLink
            v-for="link in links"
            :key="link.to"
            :to="link.to"
            class="inline-flex items-center gap-1.5 rounded-full px-4 py-2 text-[15px] transition-colors duration-150"
            :class="currentPath === link.to ? 'bg-primary text-primary-foreground' : 'text-muted-foreground hover:bg-accent hover:text-foreground'"
          >
```

改为：

```html
        <nav class="hidden items-center gap-1.5 md:flex">
          <RouterLink
            v-for="link in navLinks"
            :key="link.to"
            :to="link.to"
            class="inline-flex items-center gap-1.5 rounded-full px-4 py-2 text-[15px] transition-colors duration-150"
            :class="isActive(link.to) ? 'bg-primary text-primary-foreground' : 'text-muted-foreground hover:bg-accent hover:text-foreground'"
          >
```

在移动端菜单中把：

```html
            <RouterLink
              v-for="link in links"
              :key="link.to"
              :to="link.to"
              class="inline-flex items-center gap-2 rounded-full px-3.5 py-2 text-[15px] transition-colors"
              :class="currentPath === link.to ? 'bg-primary text-primary-foreground' : 'text-muted-foreground'"
              @click="closeMenu"
            >
```

改为：

```html
            <RouterLink
              v-for="link in navLinks"
              :key="link.to"
              :to="link.to"
              class="inline-flex items-center gap-2 rounded-full px-3.5 py-2 text-[15px] transition-colors"
              :class="isActive(link.to) ? 'bg-primary text-primary-foreground' : 'text-muted-foreground'"
              @click="closeMenu"
            >
```

页脚的 `Github` 图标（第 128 行附近）保留不动——那是仓库链接图标，与加速功能无关。

- [ ] **Step 7: 后端 SPA 路由注册 /login 与 /register**

在 `src/main.go` 的 `registerFrontendRoutes` 中，前端禁用分支里在 `router.GET("/search", notFound)` 之后补上：

```go
		router.GET("/login", notFound)
		router.GET("/register", notFound)
```

启用分支里在 `router.GET("/search", serveSPA)` 之后补上：

```go
	router.GET("/login", serveSPA)
	router.GET("/register", serveSPA)
```

- [ ] **Step 8: 修正两处仍指向旧登录页的跳转**

`/admin/login` 删除后，还有两处代码在往它跳转，必须一并改到 `/login`，否则错误提示会丢失、退出登录会落到死路由。

**(a) 后端 OAuth 错误回跳** —— `src/handlers/oauth.go` 的 `redirectOAuthError`：

```go
func redirectOAuthError(c *gin.Context, msg string) {
	c.Redirect(http.StatusFound, "/admin/login?oauth_error="+url.QueryEscape(msg))
}
```

改为：

```go
func redirectOAuthError(c *gin.Context, msg string) {
	c.Redirect(http.StatusFound, "/login?oauth_error="+url.QueryEscape(msg))
}
```

若此时仍保留旧地址，前端守卫会把 `/admin/login` 归入 `/admin` 前缀要求登录，并把用户重定向到 `/login?redirect=/admin/login`——`oauth_error` 参数在途中被丢弃，用户看不到任何失败原因。

**(b) 前端退出登录** —— `web/src/admin/AdminLayout.vue` 第 69 行附近：

```ts
  router.push('/admin/login')
```

改为：

```ts
  router.push('/login')
```

- [ ] **Step 9: 新增后端路由测试**

在 `src/handlers/oauth_test.go` 末尾追加（该文件由 Task 4 创建，此时已存在 `buildOAuthRedirect` 的测试）：

```go
func TestOAuthErrorRedirectsToLoginRoute(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	// gin.CreateTestContext 不会设置 Request，而 c.Redirect 内部的 http.Redirect 会读取它
	c.Request = httptest.NewRequest("GET", "/api/admin/oauth/callback", nil)

	redirectOAuthError(c, "state 无效或已过期")

	loc := w.Header().Get("Location")
	if !strings.HasPrefix(loc, "/login?oauth_error=") {
		t.Fatalf("Location = %q, want /login?oauth_error=...", loc)
	}
}
```

> `c.Request` 那一行是必需的：`gin.CreateTestContext` 只创建 context，不填充 `Request`，而 `c.Redirect` 会走到 `http.Redirect`，后者读取 `req.URL`。缺了它会空指针 panic。踩过一次。

并把该文件的 import 补全为：

```go
import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)
```

同时删掉 `web/src/admin/AdminLayout.vue` 中对旧登录页路径的引用。自查：

```bash
cd e:/Programming/HTML/hubproxy
grep -rn "admin/login" web/src src --include=*.vue --include=*.ts --include=*.go | grep -v "/api/admin/login"
```

Expected: 无输出。（`/api/admin/login` 是登录接口本身，保留，因此从结果中排除。）

在 `src/main_test.go` 末尾新增：

```go
func TestAuthPagesServeSPAWhenFrontendEnabled(t *testing.T) {
	router := newTestRouter(t, `
[server]
enableFrontend = true
`)

	for _, path := range []string{"/login", "/register"} {
		w := performRequest(router, http.MethodGet, path, "")
		if w.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want 200; body=%s", path, w.Code, w.Body.String())
		}
		if !strings.Contains(w.Header().Get("Content-Type"), "text/html") {
			t.Fatalf("%s content-type = %q, want text/html", path, w.Header().Get("Content-Type"))
		}
	}
}

func TestOldAdminLoginRouteIsGone(t *testing.T) {
	router := newTestRouter(t, `
[server]
enableFrontend = false
`)

	w := performRequest(router, http.MethodGet, "/admin/login", "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("/admin/login status = %d, want 404; body=%s", w.Code, w.Body.String())
	}
}
```

> 为什么这里必须关掉 `enableFrontend`：后端仍然注册着 `GET /admin/*path` → `serveSPA`，所以前端启用时 `/admin/login` 会返回 200 的 SPA 外壳。客户端路由守卫会把它当作 `/admin` 前缀路径要求登录、未登录时转到 `/login`——这是期望的用户体验。要断言「旧路由确实没了」这个后端事实，只能在前端禁用的配置下观察 NoRoute 的 404。

- [ ] **Step 10: 运行测试与构建**

Run: `cd src && go test ./... && cd ../web && npm run build`
Expected: Go 测试全绿；`vue-tsc` 无类型错误、构建成功。

- [ ] **Step 11: 提交**

```bash
cd e:/Programming/HTML/hubproxy
git add web/src/pages/AuthPage.vue web/src/router/index.ts web/src/components/AppShell.vue \
        web/src/admin/AdminLayout.vue src/main.go src/main_test.go \
        src/handlers/oauth.go src/handlers/oauth_test.go
git commit -m "feat(auth): add login/register routes and login-aware site navigation"
```

（`web/src/admin/pages/LoginPage.vue` 的删除已在 Step 2 由 `git rm` 暂存。同样**不要用 `git add -A`**。）

---

### Task 8: 端到端验证

**Files:** 无改动（纯验证）

**Interfaces:**
- Consumes: Task 1–7 的全部产物
- Produces: 一份可复现的验证结论

- [ ] **Step 1: 全量测试与构建**

```bash
cd e:/Programming/HTML/hubproxy/src && go test ./...
cd ../web && npm run build
```

Expected: 两者均无错误。

- [ ] **Step 2: 确认没有残留引用**

```bash
cd e:/Programming/HTML/hubproxy
grep -rn "GitHubProxyHandler\|CheckGitHubAccess\|ProxyGitHubRequest\|CheckGitHubURL\|contentProxy\|trackContentPull\|recordContentBytes" src --include=*.go
grep -rni "huggingface" src --include=*.go web/src
grep -rn "'/images'\|/gh/\|/hf/" web/src
grep -rn "admin/login" web/src src --include=*.vue --include=*.ts --include=*.go | grep -v "/api/admin/login"
```

Expected：
- 第一条（被删符号）：**无输出**。任何命中都是真残留。
- 第二条（`huggingface`）：会有命中，但**全部是合理留存**，预期恰为这 6 处 —— `src/db/settings_test.go:17,33`（Task 2 新增的移除回归测试与旧 JSON 兼容测试，本就以这两个字符串为断言对象与夹具内容）、`src/db/stats.go:658,661`（历史 category 聚合查询）、`src/main_test.go:274,276`（断言 `/admin/login` 已消失的测试）。除此之外的任何命中都要查。
- 第三条（`'/images'`、`/gh/`、`/hf/`）：**无输出**。
- 第四条（`admin/login`）：**无输出**（`/api/admin/login` 已被 `grep -v` 排除）。若命中，说明还有地方在跳转已删除的旧登录页。

- [ ] **Step 3: 启动服务并做拉取链路回归**

```bash
cd e:/Programming/HTML/hubproxy/src && CONFIG_PATH=./config.toml go run . &
sleep 3
curl -s -o /dev/null -w '%{http_code}\n' http://127.0.0.1:5000/v2/
```

Expected: `200`。这一步确认 Docker 拉取链路未被波及。

- [ ] **Step 4: 验证 404 收敛**

```bash
curl -s -i http://127.0.0.1:5000/gh/owner/repo/raw/main/a.txt | head -1
curl -s http://127.0.0.1:5000/nonexistent
curl -s http://127.0.0.1:5000/api/nonexistent
curl -s -i http://127.0.0.1:5000/Ab12Cd34 | grep -i 'x-robots-tag'
```

Expected：四条命令依次返回 `HTTP/1.1 404`、含 `"code":"NOT_FOUND"` 的 JSON、含 `"code":"NOT_FOUND"` 的 JSON、以及 `X-Robots-Tag: noindex, nofollow, noarchive`。

- [ ] **Step 5: 浏览器端验证导航与认证**

在浏览器中依次确认：

1. 未登录访问 `/` → 导航显示「镜像加速 / 镜像搜索 / 登录账号 / 注册账号」四项
2. 点击「注册账号」→ 进入 `/register`；若站点关闭了注册，页面显示「当前站点已关闭注册」且导航中无「注册账号」入口
3. 注册成功后跳到 `/login` 并提示「注册成功，请登录」
4. 用新账号登录 → 导航变为「镜像加速 / 镜像搜索 / 管理后台」三项
5. 已登录状态下手动访问 `/login` → 自动跳回 `/admin/user`
6. 点击「管理后台」进入控制台，侧栏「退出登录」后回到 `/` → 导航恢复为四项
7. 若配置了 OAuth：用未注册过的第三方账号登录 → 落到 `/admin/change-password` 并弹出「已自动为你创建账号：<用户名>，请先设置密码」；在此页完成改密前访问 `/admin/user` 会被挡回该页

- [ ] **Step 6: 单独提交设计文档**

规格文档与实施计划此前一直未跟踪（Task 6 / Task 7 已改用显式文件清单，没有把它们卷进代码提交）。在这里用一个独立的 `docs` 提交收尾，保持代码历史干净：

```bash
cd e:/Programming/HTML/hubproxy
git add docs/superpowers/
git commit -m "docs: add design spec and implementation plan for github/hf removal"
```

- [ ] **Step 7: 收尾**

```bash
cd e:/Programming/HTML/hubproxy
git status
```

Expected: 工作区干净。把 Step 1–5 的实际输出整理成一段验证结论，附在任务完成汇报中。
