# 移除 GitHub/Hugging Face 加速 + 认证路由重构 设计文档

日期：2026-09-19
状态：待评审

## 一、背景与目标

HubProxy 增强版目前同时承担 Docker 镜像加速与 GitHub / Hugging Face 文件加速两类职责。
GitHub / HF 侧的加速链接形态过于多样（Release、Archive、Blob、Raw、Clone、API、Gist、
Assets、Hugging Face、cdn-lfs），正则维护成本高，且与核心的镜像加速业务无关。

本次改造做四件事：

1. **彻底移除** GitHub / Hugging Face 加速能力（前后端一起删）
2. **移除** 离线镜像下载的前端页面与路由，后端能力保留
3. **重构** 顶部导航与认证路由，补上注册入口
4. **修复** OAuth 自动建号用户永不设密码的安全问题

改造后对外 URL 面收敛为：`/v2/*`（Docker 拉取）、`/api/*`（接口）、`/` `/search`
`/login` `/register` `/admin/*`（页面）。其余一律 404。

## 二、决策记录

| 议题 | 决定 | 理由 |
|---|---|---|
| 未匹配 URL（NoRoute） | 统一返回 404 JSON，删除 GitHub 代理兜底 | 语义清晰、攻击面最小；消灭「浏览器打错地址却触发代理」的隐患 |
| 已登录时的导航 | 「登录账号 / 注册账号」合并为单个「管理后台」 | 退出登录已在控制台侧栏（`AdminLayout.vue:172`），导航不重复 |
| OAuth 首次登录账号不存在 | 静默自动建号 + 回跳带标记，前端弹提示 | 用户零额外操作，同时消除「凭空多出账号」的困惑 |
| OAuth 建号用户密码 | **设 `must_change_password = 1`，强制设置密码** | 原实现给随机密码却置 0，账号永远无法用密码登录，也不会被提示 —— 实为漏洞 |
| `/admin/login` 旧路由 | **不做重定向**，直接删 | 项目迭代期不在兼容层上堆积；旧链接失效可接受 |
| `/images` 前端页面 | **整体删除**（页面 + 路由 + 导航 + 死代码） | 已验证离线打包走 `/api/image/*`，与 `/v2/*` 拉取链路完全独立 |
| 历史 `category = github/huggingface` 数据 | **不做兼容**，连带删除所有标签映射与筛选项 | 保留期默认 30 天（`retention_days`），到期自动清理；不为过渡期背历史包袱 |
| 后端离线打包 API | **保留不动** | 与本次目标无关，且未影响拉取链路 |

### 被否决的选项

- **保留通用 URL 反代入口**：需新增域名白名单与 SSRF 防护，工作量与风险高于收益。
- **保留 `/admin/login` 重定向**：仅为兼容旧书签，与「不背历史包袱」的原则相悖。
- **OAuth 首次登录跳注册确认页**：多一步交互，且需引入待注册 ticket 机制。

## 三、第一部分：后端移除 GitHub / Hugging Face

### 3.1 删除文件

| 文件 | 行数 | 内容 |
|---|---|---|
| `src/handlers/github.go` | 198 | `GitHubProxyHandler`、`CheckGitHubURL`、`ProxyGitHubRequest`、`githubExps`、`blockedContentTypes` |
| `src/handlers/content_proxy.go` | 224 | `/gh` `/hf` 令牌路径代理、`contentProxyExternalBase`、`contentProxyRedirectLocation` |
| `src/handlers/github_test.go` | 73 | 随实现删除 |

### 3.2 `src/main.go`

- 删除 4 条路由注册：`/gh/*path`、`/hf/*path`、`/:token/gh/*path`、`/:token/hf/*path`
- 删除 `api_request` 中间件：全项目仅有 `c.Set("api_request", true)`，无任何读取方，是死代码
- **重写 `NoRoute`**：
  - `/api/*` → 404 `{"error":"接口不存在","code":"NOT_FOUND","path":...,"version":...}`
  - `/TOKEN` 形式的浏览器误访问 → 404 且带 `X-Robots-Tag: noindex` 与 `Cache-Control: no-store`（**此分支必须保留**，用于防止个人令牌被搜索引擎索引，与 GitHub 无关）
  - 其余 → 404 `{"error":"页面不存在","code":"NOT_FOUND"}`
  - 删除末尾的 `handlers.GitHubProxyHandler(c)` 调用

### 3.3 `src/db/settings.go`

- `FeatureToggles` 结构体删除 `GitHub`、`HuggingFace` 两个字段
- `DefaultFeatureToggles()` 删除对应默认值 `true`
- **不写数据迁移**：旧 setting JSON 中的 `github` / `huggingface` 键在反序列化时被自动忽略，下次写入即自然消失

### 3.4 `src/handlers/auth.go`

- `AuthPublicConfig` 返回的 `features` map 删除 `github`、`huggingface` 两项

### 3.5 `src/handlers/pulltrack.go`

删除两个因本次改动失去全部调用方的函数：

- `trackContentPull` —— 唯一调用方是已删除的 `content_proxy.go`
- `recordContentBytes` —— 唯一调用方是已删除的 `github.go`

**保留** `countingWriter`、`trackDockerPull`、`recordPullBytes`、`CheckPullQuota`、`globalLimiterExempt` —— 它们仍服务于 Docker 拉取链路。

### 3.6 `src/utils/access_control.go`

- 删除 `CheckGitHubAccess`（唯一调用方在 `content_proxy.go`）
- 删除 `access_control_test.go` 中对应测试用例
- **保留** `whiteList` / `blackList` 配置与 `CheckDockerAccess` —— 黑白名单同时服务于 Docker 镜像访问控制

### 3.7 不受影响、明确不改

- `AdminPutFeatures` 整体反序列化 `FeatureToggles`，前端少传字段不会报错
- 所有 Docker 拉取链路：`/v2/*`、`/:token/v2/*`、`/token`、`NormalizeMirrorTokenPath`、`stripUserAccessToken`
- 离线打包 API：`/api/image/download`、`/api/image/info`、`/api/image/batch`

## 四、第二部分：前端移除显示

### 4.1 删除文件

- `web/src/pages/ImagesPage.vue`（离线镜像页）

### 4.2 `web/src/pages/HomePage.vue`

删除：`githubSources`、`visibleGithubSources`、`githubPatterns`、`huggingFacePatterns`、
`detectKind` 及配套的 `/gh` `/hf` 加速地址生成逻辑、`runtimeFeatures` 中的 `github`
与 `huggingface` 字段、相关模板区块。

保留：Docker 加速地址生成器、registry 列表展示、站点信息。

文案调整：
- hero 副标题 `Docker · GitHub · Hugging Face 多源加速` → `Docker 多源镜像加速`
- 移除「粘贴 GitHub / Hugging Face 原始链接」输入区及其标题

### 4.3 `web/src/pages/ImagesPage.vue` 删除的连带清理

`web/src/api.ts` 中仅被该页使用的导出全部删除：
`PrepareDownloadResponse`、`ImageInfoResponse`、`prepareSingleDownload`、`fetchImageInfo`、
`prepareBatchDownload`、`triggerDownload`。

**保留** `Repository`、`SearchResponse`、`TagInfo`、`TagPageResult`、`searchImages`、
`fetchTags` —— 搜索页 `SearchPage.vue` 仍在使用。

### 4.4 `web/src/router/index.ts`

- 删除 `/images` 路由
- 新增 `/login`、`/register`（见第五部分）

### 4.5 `web/src/admin/pages/FeaturesPage.vue`

删除「GitHub 加速」与「Hugging Face 加速」两个开关卡片。
保留「离线镜像包」开关（后端能力仍在，仅前端入口移除）。

### 4.6 `web/src/admin/pages/PullsPage.vue` / `ImagesPage.vue`

分类筛选下拉删除 `github`、`huggingface` 两个选项。

### 4.7 `web/src/admin/api.ts`

- `FeatureToggles` 接口删除 `github`、`huggingface` 字段
- 删除 `request()` 中的 `STALE_SERVER` 启发式判断 —— 该判断依据「响应含『无效输入』」识别旧进程落到了
  GitHub 代理，GitHub 代理删除后此条件永不成立
- `categoryLabel`、`pullSourceLabel` 删除 github / huggingface 分支
- `displayPullName` 删除 github / huggingface 分支，简化为 `p.tag ? image_name:tag : image_name`

> 过渡期影响：保留期内（默认 30 天）历史记录中的这两类 category 会在后台以原始字符串
> `github` / `huggingface` 展示，且无法通过筛选定位。已确认接受。

### 4.8 `web/src/lib/site.ts`

`description` 移除 `GitHub、Hugging Face` 字样。

## 五、第三部分：认证路由与导航

### 5.1 路由表

| 路径 | 组件 | 说明 |
|---|---|---|
| `/` | `HomePage` | 镜像加速 |
| `/search` | `SearchPage` | 镜像搜索 |
| `/login` | `AuthPage`（`mode: 'login'`） | 登录 |
| `/register` | `AuthPage`（`mode: 'register'`） | 注册 |
| `/admin` 及子路由 | `AdminLayout` | 控制台，保持不变 |

删除 `/admin/login` 路由，不提供重定向。

### 5.2 组件结构

**不**拆成 `LoginPage.vue` + `RegisterPage.vue` 两个文件。两者的表单、邮箱验证码、
OAuth 入口、`publicConfig` 拉取逻辑几乎完全重合，拆开立即产生重复。

做法：新建 `web/src/pages/AuthPage.vue`，两个路由指向同一组件，通过 `props: { mode }`
区分。视觉与交互沿用原 `LoginPage.vue`（卡片式布局、"返回主页"按钮、站点信息）。
原 `web/src/admin/pages/LoginPage.vue` 删除。

组件职责：
- 读 `adminApi.publicConfig()` 获取 `form_register_enabled`、`email_register_enabled`、
  `oauth_login_enabled`、`oauth.display_name`
- 接收 `route.query.oauth_error` 并展示
- 登录成功后按 `must_change_password` / `role` 跳转（沿用现有逻辑）

### 5.3 导航（`web/src/components/AppShell.vue`）

```
未登录：镜像加速 | 镜像搜索 | 登录账号 | 注册账号
已登录：镜像加速 | 镜像搜索 | 管理后台
```

实现要点：

1. `AppShell` 挂载时调用 `useAuth().bootstrap()` —— 前台页面当前完全不初始化 auth，
   这是让导航感知登录态的必要前提
2. `links` 改为 `computed`，由 `auth.isAuthed` 决定第三、四个位置的内容
3. **`form_register_enabled` 为 false 时隐藏「注册账号」** —— 否则点击进入的是必然被拒的
   页面，与「人性化」目标相悖
4. `isActive(link)` 抽成单一函数：
   - 「管理后台」→ `route.path.startsWith('/admin')`
   - 其余 → `route.path === link.to`
   桌面端与移动端菜单**共用**这份 `links` 与 `isActive()`，消除现有的两处重复
5. 「管理后台」直接链到 `/admin`。角色分流由既有 router guard 完成
   （guard 中已有 `to.path === '/admin' && role !== 'admin'` → `/admin/user`）
6. 移除 `Container`（离线镜像）图标引用，导航不再有 `/images` 入口

### 5.4 router guard 调整（`web/src/router/index.ts`）

现有 `beforeEach` 首行为 `if (!to.path.startsWith('/admin')) return true`，即**只对
`/admin/*` 生效**。`/login`、`/register` 落在 `/admin` 之外，会绕过鉴权，导致
「已登录用户访问 `/login` 不会跳回控制台」。

调整：

1. 把「已登录则弹回控制台」的判断提取为独立分支，覆盖 `/login` 与 `/register`：
   bootstrap 后若已登录，按 `must_change_password` → `/admin/change-password`、
   `role` → `/admin` 或 `/admin/user` 重定向
2. 原 `to.meta.public` 分支随 `/admin/login` 一起删除（不再有 public 的 admin 路由）
3. `/admin/*` 的鉴权、强制改密、角色分流逻辑保持原样

### 5.5 OAuth 首次登录自动注册的回跳

后端 `src/handlers/oauth.go` 的 `OAuthCallback` 末尾，按是否新建账号区分回跳：

| 情况 | 回跳 URL |
|---|---|
| 已有绑定，正常登录 | `/admin#oauth_token=<token>`（或 `/admin/user#...`、`/admin/change-password#...`） |
| 本次自动新建了账号 | `/admin/change-password#oauth_token=<token>&welcome=1` |

**必须同时修改 `web/src/main.ts`**：现有实现是

```js
const token = decodeURIComponent(window.location.hash.slice('#oauth_token='.length))
```

即把 hash 剩余部分**整段**当 token。加入 `&welcome=1` 后 token 会被污染为
`abc123&welcome=1`，导致登录失效。需改为正经解析：

- 去掉前导 `#`，按 `&` 拆分，再按第一个 `=` 拆 key/value
- `oauth_token` → `setToken(token)`
- `welcome=1` → 传递给落地页用于弹提示
- 解析后用 `history.replaceState` 清除 hash（保留现有行为）

提示内容：`已自动为你创建账号：<用户名>，请先设置密码`。
因新建的 OAuth 用户 `must_change_password = 1`，router guard 会把用户引到
`/admin/change-password`，提示与落地页一致。

**为什么用 hash 而非 query**：query 中的 token 会进入 `Referer` 头与服务器访问日志，
hash 不会发送给服务器。

### 5.6 不做的事

- 不保留 `/admin/login` 重定向
- 不在导航放「退出登录」按钮（控制台侧栏已有）
- 不做邮箱注册开关之外的注册策略改动

## 六、第四部分：OAuth 建号用户强制设密（安全修复）

`src/db/oauth.go` 的 `CreateOAuthUser`：

- 当前实现（第 141-155 行）生成随机密码后执行
  `UPDATE users SET must_change_password = 0 WHERE id = ?`
- **改为设为 1**：OAuth 建号用户首次进入控制台必须走一次「账户资料」设置可用密码

配套检查：
- `AuthRequired`（`auth.go:76-89`）已实现「`must_change_password` 为真时除
  `me` / `change-password` / `profile` / `logout` 外一律 403」，无需改动
- router guard（`router/index.ts:147-149`）已将此类用户导向 `/admin/change-password`，无需改动
- `oauth.go:313-315` 已处理 `MustChangePassword` 时的跳转目标，无需改动

副作用：已存在的 OAuth 建号用户不受影响（不回溯修改），仅新建用户适用。

## 七、影响面

### 后端

| 文件 | 动作 |
|---|---|
| `src/handlers/github.go` | 删除 |
| `src/handlers/content_proxy.go` | 删除 |
| `src/handlers/github_test.go` | 删除 |
| `src/main.go` | 改：删路由、重写 NoRoute、删死中间件 |
| `src/db/settings.go` | 改：FeatureToggles 减字段 |
| `src/db/oauth.go` | 改：must_change_password = 1 |
| `src/handlers/auth.go` | 改：publicConfig features 减字段 |
| `src/handlers/pulltrack.go` | 改：删 trackContentPull、recordContentBytes |
| `src/handlers/oauth.go` | 改：回跳加 welcome 标记 |
| `src/utils/access_control.go` | 改：删 CheckGitHubAccess |
| `src/utils/access_control_test.go` | 改：删对应用例 |

### 前端

| 文件 | 动作 |
|---|---|
| `web/src/pages/ImagesPage.vue` | 删除 |
| `web/src/pages/AuthPage.vue` | 新增 |
| `web/src/admin/pages/LoginPage.vue` | 删除 |
| `web/src/pages/HomePage.vue` | 改：删 GitHub/HF |
| `web/src/components/AppShell.vue` | 改：导航按登录态切换 |
| `web/src/router/index.ts` | 改：路由表重构 |
| `web/src/main.ts` | 改：hash 解析 |
| `web/src/api.ts` | 改：删离线下载相关导出 |
| `web/src/admin/api.ts` | 改：删字段、删启发式、删标签分支 |
| `web/src/admin/pages/FeaturesPage.vue` | 改：删两个开关 |
| `web/src/admin/pages/PullsPage.vue` | 改：删筛选项 |
| `web/src/admin/pages/ImagesPage.vue` | 改：删筛选项 |
| `web/src/lib/site.ts` | 改：文案 |

## 八、验证

1. `cd src && go test ./...` 全部通过
2. `cd web && npm run build` 通过（`vue-tsc` 会捕获所有残留引用）
3. 新增 `src/main_test.go` 用例，断言：
   - `GET /gh/owner/repo/raw/main/a.txt` → 404
   - `GET /hf/owner/model/resolve/main/x.bin` → 404
   - `GET /nonexistent` → 404 且响应体为 JSON 而非纯文本
   - `GET /api/nonexistent` → 404 且 `code` 为 `NOT_FOUND`
4. 手动验证（`docker pull` 链路回归）：
   - `curl -H "Authorization: ..." http://127.0.0.1:5000/v2/` 返回 200
   - `docker pull 127.0.0.1:5000/<TOKEN>/library/nginx:latest` 正常拉取
   - `GET /TOKEN` 返回 404 且带 `X-Robots-Tag: noindex`
5. 手动验证（认证）：
   - 未登录访问 `/`、`/search` → 导航显示 4 个入口
   - 登录后 → 显示「管理后台」，点击进入控制台
   - 注册关闭时 → 导航不出现「注册账号」，直接访问 `/register` 显示关闭提示
   - 已登录状态下直接访问 `/login` → 自动跳回控制台（guard 生效）
   - OAuth 首次登录 → 自动建号，落到 `/admin/change-password` 并弹出提示，
     且未设置新密码前无法调用其他业务 API

## 九、明确不做

- 不新增通用 URL 反向代理能力
- 不保留 `/gh` `/hf` 路由的任何形式（含返回空响应的占位）
- 不回溯修改已存在的 OAuth 用户密码策略
- 不改动 `/v2/*` 拉取链路、离线打包后端 API、统计与配额逻辑
- 不做历史 category 数据的迁移或清理（等待 `retention_days` 自然过期）
