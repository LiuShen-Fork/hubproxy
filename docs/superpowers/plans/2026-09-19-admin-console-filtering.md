# 管理后台筛选与表格优化 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 给管理后台的全局拉取、镜像统计、IP 分析三个页面补上用户/时间段/来源/类型筛选，表格显示用户信息并做呈现精化，同时删掉「层/请求」列。

**Architecture:** 数据层所需的列全部已存在，因此**没有 schema 变更**——所有工作都是只读查询与新字段。后端先把 `PullListFilter` 的单用户改成多用户、把用户名解析移到服务端、加一个 IP 前缀建议端点；前端随后加三个无依赖的小组件（多选、自动补全、日期区间），三个页面各自组合它们。

**Tech Stack:** Go 1.26 + Gin、SQLite（modernc.org/sqlite）、Vue 3 + TypeScript + Tailwind v4。

**Spec:** `docs/superpowers/specs/2026-09-19-admin-console-filtering-design.md`

## Global Constraints

- **不引入任何前端依赖。** 现有依赖只有 vue、vue-router、lucide-vue-next、clsx、tailwind-merge 与字体。三个新组件用现有 `components/ui/*` 的模式手写。
- **不做数据库结构变更、不做数据回填。** 用户通过替换 Docker 镜像升级并沿用同一个 SQLite 文件。所有新查询只读。
- **`pull_sessions.user_id` 可为 NULL**（匿名拉取，以及该列存在之前的历史行）。任何 JOIN **必须是 LEFT JOIN**，任何筛选都不得把 NULL 行误删。展示为「匿名」。
- **不改动 `layer_count` / `request_count` 的存储、API 返回与计数逻辑。** 「层/请求」只从界面上删掉。
- **不加用户列到镜像统计页**（一个镜像可被多人拉取）。
- **多值筛选参数用重复形式**：`?user_id=2&user_id=5`，服务端用 `c.QueryArray("user_id")` 读取。不支持逗号分隔。
- 后端每个任务结束时 `cd src && go test ./...` 必须全绿（当前基线为 0 失败）。
- 前端验证手段是 `cd web && npm run build`（含 `vue-tsc`），**不得引入测试框架**。
- 注释与用户可见文案一律中文，与既有风格一致。
- 本环境**没有浏览器**：前端任务的最终观感需人工确认，实施者不得声称已在浏览器验证。

---

### Task 1: 后端 — 多用户筛选

`PullListFilter` 目前是单值 `UserID int64`，SQL 为 `user_id = ?`。改为多值 `UserIDs []int64`，SQL 用 `IN`。有三处调用方传 `UserID`，改类型后它们会编译失败——这是安全的失败方式，逐个修即可。

**Files:**
- Modify: `src/db/stats.go`（`PullListFilter` 定义、`ListPullSessions` 的 WHERE 构造、第 356 行附近的 `UserDashboard`）
- Modify: `src/handlers/admin.go`（`AdminListPulls`）
- Modify: `src/handlers/user_console.go`（`UserListPulls`）
- Test: `src/db/stats_test.go`（已有文件，追加）

**Interfaces:**
- Consumes: 无
- Produces: `db.PullListFilter.UserIDs []int64`（空切片 = 不按用户筛选）。后续任务的 handler 从 `c.QueryArray("user_id")` 解析后经 `strconv.ParseInt` 转成 `[]int64` 传入。

- [ ] **Step 1: 写失败测试**

在 `src/db/stats_test.go` 末尾追加。该文件的测试用 `useTestDB(t)` + `migrate()`，与 `src/db/user_tokens_test.go` 里的模式一致（两个 helper 都定义在 `db` 包内）。

```go
// insertPullSession 插入一条拉取记录，userID 为 0 表示匿名为 NULL。
func insertPullSession(t *testing.T, id, ip, image string, userID int64) {
	t.Helper()
	now := Now()
	var uid any
	if userID > 0 {
		uid = userID
	}
	if _, err := DB.Exec(
		`INSERT INTO pull_sessions
		 (id, client_ip, image_name, registry, tag, category, started_at, last_seen_at,
		  status, bytes_total, layer_count, request_count, user_id)
		 VALUES (?, ?, ?, 'docker.io', 'latest', 'library', ?, ?, 'completed', 100, 1, 1, ?)`,
		id, ip, image, now, now, uid,
	); err != nil {
		t.Fatalf("insert pull session: %v", err)
	}
}

func TestListPullSessionsFiltersByMultipleUsers(t *testing.T) {
	useTestDB(t)
	if err := migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	now := Now()
	for _, u := range []struct {
		id       int64
		username string
	}{{1, "alice"}, {2, "bob"}, {3, "carol"}} {
		if _, err := DB.Exec(
			`INSERT INTO users (id, username, password_hash, role, created_at, updated_at)
			 VALUES (?, ?, 'hash', 'user', ?, ?)`, u.id, u.username, now, now,
		); err != nil {
			t.Fatalf("insert user: %v", err)
		}
	}
	insertPullSession(t, "s-alice", "10.0.0.1", "alpine", 1)
	insertPullSession(t, "s-bob", "10.0.0.2", "nginx", 2)
	insertPullSession(t, "s-carol", "10.0.0.3", "redis", 3)

	got, total, err := ListPullSessions(PullListFilter{
		UserIDs: []int64{1, 3}, Page: 1, PageSize: 50,
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 2 || len(got) != 2 {
		t.Fatalf("选中两个用户应恰好返回 2 条，实际 total=%d len=%d", total, len(got))
	}
	for _, s := range got {
		if s.UserID != 1 && s.UserID != 3 {
			t.Fatalf("返回了未被选中的用户 %d", s.UserID)
		}
	}
}

func TestListPullSessionsEmptyUserFilterReturnsAll(t *testing.T) {
	useTestDB(t)
	if err := migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	insertPullSession(t, "s-anon", "10.0.0.9", "busybox", 0)

	// 空切片必须表示「不筛选」，而不是「筛选出 user_id 为空的」
	got, total, err := ListPullSessions(PullListFilter{Page: 1, PageSize: 50})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 1 || len(got) != 1 {
		t.Fatalf("空筛选条件应返回全部 1 条，实际 total=%d len=%d", total, len(got))
	}
	if got[0].UserID != 0 {
		t.Fatalf("匿名行的 UserID 应为 0，实际 %d", got[0].UserID)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd src && go test ./db/ -run 'TestListPullSessionsFiltersByMultipleUsers|TestListPullSessionsEmptyUserFilterReturnsAll' -v`
Expected: 编译失败并报 `unknown field 'UserIDs' in struct literal of type PullListFilter`。这就是本步要的 RED——字段还不存在。

- [ ] **Step 3: 改 `PullListFilter` 与 `ListPullSessions`**

在 `src/db/stats.go` 中把 `PullListFilter` 的 `UserID int64` 改为：

```go
	// UserIDs 为空表示不按用户筛选；非空时用 IN 匹配。
	// 注意 user_id 可为 NULL（匿名拉取），NULL 行永远不匹配任何 UserIDs。
	UserIDs []int64
```

把 `ListPullSessions` 里这段：

```go
	if f.UserID > 0 {
		where = append(where, "user_id = ?")
		args = append(args, f.UserID)
	}
```

改为：

```go
	if len(f.UserIDs) > 0 {
		placeholders := make([]string, len(f.UserIDs))
		for i, id := range f.UserIDs {
			placeholders[i] = "?"
			args = append(args, id)
		}
		where = append(where, "user_id IN ("+strings.Join(placeholders, ",")+")")
	}
```

- [ ] **Step 4: 修三处调用方**

`src/db/stats.go` 第 356 行附近（`UserDashboard` 里）：

```go
	recent, _, _ := ListPullSessions(PullListFilter{UserID: userID, Page: 1, PageSize: 30, CountedOnly: true})
```

改为：

```go
	recent, _, _ := ListPullSessions(PullListFilter{UserIDs: []int64{userID}, Page: 1, PageSize: 30, CountedOnly: true})
```

`src/handlers/user_console.go` 的 `UserListPulls`（第 41 行附近）里那处 `PullListFilter{...}` 同样把 `UserID: u.ID` 改为 `UserIDs: []int64{u.ID}`（保留该处其余字段不动）。

`src/handlers/admin.go` 的 `AdminListPulls` 里那处 `PullListFilter{...}` 增加一个字段：

```go
		UserIDs:     parseUserIDs(c.QueryArray("user_id")),
```

并在同文件末尾新增解析辅助：

```go
// parseUserIDs 解析重复出现的 user_id 参数（?user_id=2&user_id=5）。
// 无法解析的项直接跳过，不因为一个坏值让整个筛选失败。
func parseUserIDs(raw []string) []int64 {
	out := make([]int64, 0, len(raw))
	for _, s := range raw {
		n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
		if err != nil || n <= 0 {
			continue
		}
		out = append(out, n)
	}
	return out
}
```

- [ ] **Step 5: 运行测试确认通过**

Run: `cd src && go test ./...`
Expected: PASS，全部包 ok。

- [ ] **Step 6: 提交**

```bash
git add src/db/stats.go src/handlers/admin.go src/handlers/user_console.go src/db/stats_test.go
git commit -m "feat(stats): filter pull sessions by multiple users"
```

---

### Task 2: 后端 — 用户名在服务端解析

表格要显示用户，`pull_sessions` 只存了 `user_id`。同时 IP 分析需要「一个 IP 关联了哪些用户」。两者都在服务端解析，前端不必维护 id→名字映射。

**Files:**
- Modify: `src/db/stats.go`（`PullSession` 结构、`ListPullSessions` 查询、`IPStat` 结构、`ListIPStats`）
- Test: `src/db/stats_test.go`（追加）

**Interfaces:**
- Consumes: Task 1 的 `PullListFilter.UserIDs`
- Produces:
  - `PullSession.Username string`（JSON `username`，匿名为空串）
  - `IPStat.Users []string`（JSON `users`，已排序，匿名不产生条目）

- [ ] **Step 1: 写失败测试**

在 `src/db/stats_test.go` 末尾追加。`insertPullSession` 来自 Task 1。

```go
func TestListPullSessionsResolvesUsernameAndKeepsAnonymous(t *testing.T) {
	useTestDB(t)
	if err := migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	now := Now()
	if _, err := DB.Exec(
		`INSERT INTO users (id, username, password_hash, role, created_at, updated_at)
		 VALUES (1, 'alice', 'hash', 'user', ?, ?)`, now, now,
	); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	insertPullSession(t, "s-alice", "10.0.0.1", "alpine", 1)
	insertPullSession(t, "s-anon", "10.0.0.2", "busybox", 0)

	got, _, err := ListPullSessions(PullListFilter{Page: 1, PageSize: 50})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	byID := map[string]PullSession{}
	for _, s := range got {
		byID[s.ID] = s
	}
	if byID["s-alice"].Username != "alice" {
		t.Fatalf("s-alice 的用户名应为 alice，实际 %q", byID["s-alice"].Username)
	}
	if _, ok := byID["s-anon"]; !ok {
		t.Fatal("匿名行被 INNER JOIN 弄丢了——必须用 LEFT JOIN 保留")
	}
	if byID["s-anon"].Username != "" {
		t.Fatalf("匿名行的用户名应为空，实际 %q", byID["s-anon"].Username)
	}
}

func TestListIPStatsAggregatesUsersPerIP(t *testing.T) {
	useTestDB(t)
	if err := migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	now := Now()
	for _, u := range []struct {
		id   int64
		name string
	}{{1, "alice"}, {2, "bob"}} {
		if _, err := DB.Exec(
			`INSERT INTO users (id, username, password_hash, role, created_at, updated_at)
			 VALUES (?, ?, 'hash', 'user', ?, ?)`, u.id, u.name, now, now,
		); err != nil {
			t.Fatalf("insert user: %v", err)
		}
	}
	// 同一个 IP 被两个用户使用——这正是要暴露的滥用信号
	insertPullSession(t, "s-1", "10.0.0.7", "alpine", 1)
	insertPullSession(t, "s-2", "10.0.0.7", "nginx", 2)
	// 另一个 IP 只有匿名
	insertPullSession(t, "s-3", "10.0.0.8", "busybox", 0)

	list, _, err := ListIPStats("", 1, 50)
	if err != nil {
		t.Fatalf("list ips: %v", err)
	}
	byIP := map[string]IPStat{}
	for _, it := range list {
		byIP[it.ClientIP] = it
	}
	shared := byIP["10.0.0.7"]
	if len(shared.Users) != 2 || shared.Users[0] != "alice" || shared.Users[1] != "bob" {
		t.Fatalf("10.0.0.7 应关联 alice 与 bob，实际 %#v", shared.Users)
	}
	if len(byIP["10.0.0.8"].Users) != 0 {
		t.Fatalf("纯匿名的 IP 不应有用户，实际 %#v", byIP["10.0.0.8"].Users)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd src && go test ./db/ -run 'TestListPullSessionsResolvesUsernameAndKeepsAnonymous|TestListIPStatsAggregatesUsersPerIP' -v`
Expected: 编译失败，报 `byID["s-alice"].Username undefined` 与 `it.Users undefined`。字段尚不存在。

- [ ] **Step 3: 给 `PullSession` 加字段并改查询**

`src/db/stats.go` 的 `PullSession` 结构体末尾（`UserID` 之后）加：

```go
	// Username 由服务端 LEFT JOIN users 解析；匿名为空串，用户已删除为 "#<id>"
	Username string `json:"username"`
```

把 `ListPullSessions` 的列表查询改为带 LEFT JOIN。原来是：

```go
	query := fmt.Sprintf(
		`SELECT id, client_ip, image_name, registry, tag, category, started_at, last_seen_at,
		        COALESCE(completed_at,''), status, bytes_total, layer_count, request_count,
		        COALESCE(user_id,0)
		 FROM pull_sessions WHERE %s
		 ORDER BY started_at DESC LIMIT ? OFFSET ?`, clause,
	)
```

改为：

```go
	// LEFT JOIN 而不是 INNER JOIN：匿名行与指向已删除用户的行都必须留下
	query := fmt.Sprintf(
		`SELECT p.id, p.client_ip, p.image_name, p.registry, p.tag, p.category,
		        p.started_at, p.last_seen_at, COALESCE(p.completed_at,''), p.status,
		        p.bytes_total, p.layer_count, p.request_count, COALESCE(p.user_id,0),
		        COALESCE(u.username, '')
		 FROM pull_sessions p
		 LEFT JOIN users u ON u.id = p.user_id
		 WHERE %s
		 ORDER BY p.started_at DESC LIMIT ? OFFSET ?`, clause,
	)
```

注意 `WHERE` 子句里现有的列名（`client_ip`、`image_name`、`category`、`registry`、`status`、`started_at`、`user_id`）在加了表别名后会产生歧义——`users` 表没有这些列，SQLite 能自行解析，但为清晰起见把它们统一加上 `p.` 前缀。**这一步只改 `ListPullSessions` 内部的 WHERE 构造，`PullListFilter` 结构体不变。**

即把 Step 3 之前那些 `where = append(where, "...")` 的字面量统一改为带 `p.` 前缀，例如 `"client_ip LIKE ?"` → `"p.client_ip LIKE ?"`，`"user_id IN (...)"` → `"p.user_id IN (...)"`。计数查询 `SELECT COUNT(*) FROM pull_sessions WHERE ...` 同样改为 `FROM pull_sessions p`。

扫描列表相应增加一个目标：

```go
		if err := rows.Scan(
			&s.ID, &s.ClientIP, &s.ImageName, &s.Registry, &s.Tag, &s.Category,
			&s.StartedAt, &s.LastSeenAt, &s.CompletedAt, &s.Status,
			&s.BytesTotal, &s.LayerCount, &s.RequestCount, &s.UserID, &s.Username,
		); err != nil {
```

- [ ] **Step 4: 给 `IPStat` 加字段并在 `ListIPStats` 里聚合**

`src/db/stats.go` 的 `IPStat` 结构体加：

```go
	// Users 是该 IP 关联过的用户名（去重、已排序）。匿名拉取不产生条目。
	// 同一 IP 出现多个用户是值得注意的信号，前端会据此高亮。
	Users []string `json:"users"`
```

在 `ListIPStats` 的 `for rows.Next()` 循环结束后、`if list == nil` 之前插入聚合。注意 `list` 已填充，下面用它的 IP 集合做一次批量查询：

```go
	if err := attachUsersToIPStats(list); err != nil {
		return nil, 0, err
	}
```

并在同文件新增（放在 `ListIPStats` 之后）：

```go
// attachUsersToIPStats 为当前页的每个 IP 补出关联用户名。
// 只查这一页的 IP，避免为分页列表把整表扫一遍。
func attachUsersToIPStats(list []IPStat) error {
	if len(list) == 0 {
		return nil
	}
	placeholders := make([]string, len(list))
	args := make([]any, len(list))
	for i, it := range list {
		placeholders[i] = "?"
		args[i] = it.ClientIP
	}
	rows, err := DB.Query(
		`SELECT p.client_ip, u.username
		 FROM pull_sessions p
		 JOIN users u ON u.id = p.user_id
		 WHERE p.user_id IS NOT NULL AND p.client_ip IN (`+strings.Join(placeholders, ",")+`)
		 GROUP BY p.client_ip, u.username
		 ORDER BY p.client_ip, u.username`, args...,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	byIP := map[string][]string{}
	for rows.Next() {
		var ip, name string
		if err := rows.Scan(&ip, &name); err != nil {
			return err
		}
		byIP[ip] = append(byIP[ip], name)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for i := range list {
		if names := byIP[list[i].ClientIP]; len(names) > 0 {
			list[i].Users = names
		}
	}
	return nil
}
```

这里的 `JOIN`（而非 LEFT JOIN）是**正确**的：只关心有用户的组合，匿名行本就不该产生条目。

- [ ] **Step 5: 运行测试确认通过**

Run: `cd src && go test ./...`
Expected: PASS。

- [ ] **Step 6: 提交**

```bash
git add src/db/stats.go src/db/stats_test.go
git commit -m "feat(stats): resolve usernames server-side for pull and IP lists"
```

---

### Task 3: 后端 — IP 前缀建议端点

IP 筛选要从「手输完整 IP」变成「输入前几位就给出候选」。

**Files:**
- Modify: `src/db/stats.go`（新增 `SuggestIPs`）
- Modify: `src/handlers/admin.go`（路由与处理器）
- Test: `src/db/stats_test.go`（追加）

**Interfaces:**
- Consumes: 无
- Produces: `db.SuggestIPs(prefix string, limit int) ([]string, error)`；HTTP `GET /api/admin/ips/suggest?prefix=` → `{"items": ["1.2.3.4", ...]}`

- [ ] **Step 1: 写失败测试**

在 `src/db/stats_test.go` 末尾追加：

```go
func TestSuggestIPsMatchesPrefixAndCapsResults(t *testing.T) {
	useTestDB(t)
	if err := migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	insertPullSession(t, "s-a", "10.1.1.1", "alpine", 0)
	insertPullSession(t, "s-b", "10.1.1.2", "alpine", 0)
	insertPullSession(t, "s-c", "10.1.2.1", "alpine", 0)
	insertPullSession(t, "s-d", "192.168.1.1", "alpine", 0)

	got, err := SuggestIPs("10.1.1", 10)
	if err != nil {
		t.Fatalf("suggest: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("前缀 10.1.1 应匹配 2 个 IP，实际 %#v", got)
	}

	// 上限必须生效
	got, err = SuggestIPs("10.1.", 1)
	if err != nil {
		t.Fatalf("suggest: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("limit=1 时应只返回 1 个，实际 %#v", got)
	}

	// 空前缀返回空数组，而不是把全部 IP 倒出来
	got, err = SuggestIPs("", 10)
	if err != nil {
		t.Fatalf("suggest: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("空前缀应返回空数组，实际 %#v", got)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd src && go test ./db/ -run TestSuggestIPsMatchesPrefixAndCapsResults -v`
Expected: 编译失败，`undefined: SuggestIPs`。

- [ ] **Step 3: 实现 `SuggestIPs`**

在 `src/db/stats.go` 的 `attachUsersToIPStats` 之后新增：

```go
// SuggestIPs 返回以 prefix 开头的 IP，供前端自动补全使用。
// 前缀为空时返回空数组——否则一次空输入就会把整张表的 IP 倒给前端。
func SuggestIPs(prefix string, limit int) ([]string, error) {
	out := []string{}
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return out, nil
	}
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	rows, err := DB.Query(
		`SELECT DISTINCT client_ip FROM pull_sessions
		 WHERE client_ip LIKE ? ORDER BY client_ip LIMIT ?`,
		prefix+"%", limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var ip string
		if err := rows.Scan(&ip); err != nil {
			return nil, err
		}
		out = append(out, ip)
	}
	return out, rows.Err()
}
```

- [ ] **Step 4: 接上 HTTP 层**

在 `src/handlers/admin.go` 的路由组里、`adminOnly.GET("/ips", AdminListIPs)` 之后加一行：

```go
		adminOnly.GET("/ips/suggest", AdminSuggestIPs)
```

在同文件新增处理器（放在 `AdminListIPs` 之后）：

```go
// AdminSuggestIPs 供筛选框的自动补全使用。只回前缀匹配的少量 IP。
func AdminSuggestIPs(c *gin.Context) {
	items, err := db.SuggestIPs(c.Query("prefix"), 10)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}
```

路由顺序说明：gin 中 `/ips` 与 `/ips/suggest` 是两条静态路径，不冲突；`/ips/suggest` 必须注册在 `adminOnly` 组内以继承鉴权。

- [ ] **Step 5: 运行测试确认通过**

Run: `cd src && go test ./...`
Expected: PASS。

- [ ] **Step 6: 提交**

```bash
git add src/db/stats.go src/handlers/admin.go src/db/stats_test.go
git commit -m "feat(admin): add IP prefix suggestion endpoint"
```

---

### Task 4: 后端 — 镜像统计与 IP 分析支持时间段

`ListImageStats` 与 `ListIPStats` 目前都不接受时间范围。两者都加 from/to，与 `ListPullSessions` 的语义保持一致（空串 = 不筛）。

**Files:**
- Modify: `src/db/stats.go`（`ListImageStats`、`ListIPStats`）
- Modify: `src/handlers/admin.go`（`AdminListImages`、`AdminListIPs`）
- Test: `src/db/stats_test.go`（追加）

**Interfaces:**
- Consumes: 无
- Produces:
  - `db.ListImageStats(image, category, registry, from, to string, page, pageSize int) ([]ImageStat, int, error)`
  - `db.ListIPStats(ip, from, to string, page, pageSize int) ([]IPStat, int, error)`

- [ ] **Step 1: 写失败测试**

在 `src/db/stats_test.go` 末尾追加。该测试直接写 `started_at` 以控制时间：

```go
func TestListImageStatsFiltersByTimeRange(t *testing.T) {
	useTestDB(t)
	if err := migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	insert := func(id, image, startedAt string) {
		t.Helper()
		if _, err := DB.Exec(
			`INSERT INTO pull_sessions
			 (id, client_ip, image_name, registry, tag, category, started_at, last_seen_at,
			  status, bytes_total, layer_count, request_count)
			 VALUES (?, '10.0.0.1', ?, 'docker.io', 'latest', 'library', ?, ?, 'completed', 10, 1, 1)`,
			id, image, startedAt, startedAt,
		); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}
	insert("s-old", "old/app", "2026-01-01T00:00:00Z")
	insert("s-new", "new/app", "2026-09-01T00:00:00Z")

	got, total, err := ListImageStats("", "", "", "2026-06-01T00:00:00Z", "", 1, 50)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 1 || len(got) != 1 || got[0].ImageName != "new/app" {
		t.Fatalf("按起始时间筛选后应只剩 new/app，实际 total=%d %#v", total, got)
	}

	got, total, err = ListImageStats("", "", "", "", "", 1, 50)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 2 {
		t.Fatalf("不传时间范围应返回全部 2 条，实际 %d", total)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd src && go test ./db/ -run TestListImageStatsFiltersByTimeRange -v`
Expected: 编译失败，`too many arguments in call to ListImageStats`。

- [ ] **Step 3: 改签名与 WHERE**

`src/db/stats.go` 中把：

```go
func ListImageStats(image, category, registry string, page, pageSize int) ([]ImageStat, int, error) {
```

改为：

```go
func ListImageStats(image, category, registry, from, to string, page, pageSize int) ([]ImageStat, int, error) {
```

在 `if registry != ""` 那个分支之后加：

```go
	if from != "" {
		where = append(where, "started_at >= ?")
		args = append(args, from)
	}
	if to != "" {
		where = append(where, "started_at <= ?")
		args = append(args, to)
	}
```

- [ ] **Step 4: 同样给 `ListIPStats` 加时间范围**

`src/db/stats.go` 中把：

```go
func ListIPStats(ip string, page, pageSize int) ([]IPStat, int, error) {
```

改为：

```go
func ListIPStats(ip, from, to string, page, pageSize int) ([]IPStat, int, error) {
```

该函数目前用单个字符串 `where := "1=1"` 而非切片。把它改为切片拼装，保持与 `ListImageStats` 一致：

```go
	where := []string{"1=1"}
	args := []any{}
	if ip != "" {
		where = append(where, "client_ip LIKE ?")
		args = append(args, "%"+ip+"%")
	}
	if from != "" {
		where = append(where, "started_at >= ?")
		args = append(args, from)
	}
	if to != "" {
		where = append(where, "started_at <= ?")
		args = append(args, to)
	}
	clause := strings.Join(where, " AND ")
```

并把函数体内两处原有的 `where` 用法替换为 `clause`（计数查询里的 `WHERE "+where+"` 与主查询里的 `WHERE "+where+"`，注意计数查询是 `FROM pull_sessions WHERE ...`，主查询是 `FROM pull_sessions WHERE ...`）。

- [ ] **Step 5: 追加 IP 时间范围测试**

在 `src/db/stats_test.go` 末尾追加：

```go
func TestListIPStatsFiltersByTimeRange(t *testing.T) {
	useTestDB(t)
	if err := migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	insert := func(id, ip, startedAt string) {
		t.Helper()
		if _, err := DB.Exec(
			`INSERT INTO pull_sessions
			 (id, client_ip, image_name, registry, tag, category, started_at, last_seen_at,
			  status, bytes_total, layer_count, request_count)
			 VALUES (?, ?, 'app', 'docker.io', 'latest', 'user', ?, ?, 'completed', 10, 1, 1)`,
			id, ip, startedAt, startedAt,
		); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}
	insert("s-1", "10.9.9.1", "2026-01-01T00:00:00Z")
	insert("s-2", "10.9.9.2", "2026-09-01T00:00:00Z")

	list, total, err := ListIPStats("", "2026-06-01T00:00:00Z", "", 1, 50)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 1 || len(list) != 1 || list[0].ClientIP != "10.9.9.2" {
		t.Fatalf("按起始时间筛选后应只剩 10.9.9.2，实际 total=%d %#v", total, list)
	}
}
```

- [ ] **Step 6: 改两个处理器调用**

`src/handlers/admin.go` 的 `AdminListImages` 里：

```go
	list, total, err := db.ListImageStats(c.Query("image"), c.Query("category"), c.Query("registry"), page, pageSize)
```

改为：

```go
	list, total, err := db.ListImageStats(
		c.Query("image"), c.Query("category"), c.Query("registry"),
		c.Query("from"), c.Query("to"), page, pageSize,
	)
```

同文件 `AdminListIPs` 里：

```go
	list, total, err := db.ListIPStats(c.Query("ip"), page, pageSize)
```

改为：

```go
	list, total, err := db.ListIPStats(c.Query("ip"), c.Query("from"), c.Query("to"), page, pageSize)
```

- [ ] **Step 7: 运行测试确认通过**

Run: `cd src && go test ./...`
Expected: PASS。

- [ ] **Step 8: 提交**

```bash
git add src/db/stats.go src/handlers/admin.go src/db/stats_test.go
git commit -m "feat(stats): support time range on image and IP stats"
```

---

### Task 5: 前端 — API 层与类型

`adminApi.pulls()` 目前用 `Record<string, string | number | undefined>` 拼参数，**无法发送重复参数**，多用户筛选会失败。同时补上 suggest 方法与新字段类型。

**Files:**
- Modify: `web/src/admin/api.ts`
- Create: `web/src/lib/dateRange.ts`

**Interfaces:**
- Consumes: Task 1–4 的后端
- Produces:
  - `QueryValue = string | number | undefined | Array<string | number>`，`buildQuery(q)` 对数组值重复 append 同名参数
  - `PullSession.username: string`
  - `IPStat` 类型（新导出）：`{ client_ip, pull_count, bytes_total, last_seen, users: string[] }`
  - `adminApi.ipsSuggest(prefix: string)`
  - `web/src/lib/dateRange.ts` 导出 `presetRange(preset, now?)` 与 `dateInputToRFC3339(dateStr, endOfDay)`

- [ ] **Step 1: 新增日期区间工具**

创建 `web/src/lib/dateRange.ts`：

```ts
export type DateRangePreset = 'today' | '7d' | '30d' | 'all' | 'custom'

/** 数据库里 started_at 存的是 UTC 的 RFC3339，所以本地日期必须先转成 UTC 时刻。 */
export function dateInputToRFC3339(dateStr: string, endOfDay: boolean): string {
  if (!dateStr) return ''
  const [y, m, d] = dateStr.split('-').map(Number)
  if (!y || !m || !d) return ''
  const dt = endOfDay
    ? new Date(y, m - 1, d, 23, 59, 59, 999)
    : new Date(y, m - 1, d, 0, 0, 0, 0)
  return dt.toISOString()
}

/** 把预设区间换算成 from/to；'all' 返回两个空串，即不筛选。 */
export function presetRange(
  preset: DateRangePreset,
  now: Date = new Date(),
): { from: string; to: string } {
  if (preset === 'all' || preset === 'custom') return { from: '', to: '' }
  const start = new Date(now.getFullYear(), now.getMonth(), now.getDate(), 0, 0, 0, 0)
  if (preset === '7d') start.setDate(start.getDate() - 6)
  if (preset === '30d') start.setDate(start.getDate() - 29)
  return { from: start.toISOString(), to: '' }
}
```

说明：`to` 一律留空表示「到现在」，只有自定义区间才会用到上界；近 7 天按「含今天在内的 7 个自然日」计算，所以减 6 天而不是 7 天。

- [ ] **Step 2: 改 api.ts 的参数构造**

在 `web/src/admin/api.ts` 里，把 `pulls`/`images`/`ips` 三处重复的 URLSearchParams 拼装抽成一个函数。在 `request` 函数之后新增：

```ts
/** 数组值会重复 append 同名参数（?user_id=2&user_id=5），后端用 QueryArray 读取。 */
export type QueryValue = string | number | undefined | Array<string | number>

export function buildQuery(q: Record<string, QueryValue>): string {
  const sp = new URLSearchParams()
  Object.entries(q).forEach(([k, v]) => {
    if (v === undefined || v === '') return
    if (Array.isArray(v)) {
      v.forEach((item) => {
        if (item !== undefined && item !== '') sp.append(k, String(item))
      })
      return
    }
    sp.set(k, String(v))
  })
  return sp.toString()
}
```

- [ ] **Step 3: 让三个列表方法使用它**

把 `pulls` 改为：

```ts
  pulls: (q: Record<string, QueryValue>) =>
    request<{ items: PullSession[]; total: number; page: number; page_size: number }>(
      `/pulls?${buildQuery(q)}`,
    ),
```

`images` 与 `ips` 同样改为接受 `Record<string, QueryValue>` 并使用 `buildQuery`，返回值类型由 `any[]` 改为 Step 4 定义的具体接口：

```ts
  images: (q: Record<string, QueryValue>) =>
    request<{ items: ImageStat[]; total: number }>(`/images?${buildQuery(q)}`),
  ips: (q: Record<string, QueryValue>) =>
    request<{ items: IPStat[]; total: number }>(`/ips?${buildQuery(q)}`),
```

注意 `ImageStat` / `IPStat` 定义在 Step 4，若先做 Step 3 会因类型未定义而构建失败——**按 Step 3 → Step 4 的顺序做即可，构建验证在 Step 5**。

- [ ] **Step 4: 加类型与 suggest 方法**

`PullSession` 接口末尾加**两个**字段：

```ts
  // 匿名行不会带这个键（Go 侧的 json tag 是 omitempty），故此字段是 undefined 而非 0。
  // 前端据此区分「匿名」（无此键）与「用户已删除」（有值但 username 为空串）——
  // 后者显示为 #<id>。
  user_id?: number
  username: string
```

**注意**：原计划误以为接口里已有 `user_id`，实际没有（Go 结构体有、前端接口一直没有）。后一个任务会读取 `p.user_id` 与 `selected.session.user_id`，缺了它 `vue-tsc` 会报「Property 'user_id' does not exist on type 'PullSession'」而构建失败。

在 `FeatureToggles` 之前新增两个接口：

```ts
export interface IPStat {
  client_ip: string
  pull_count: number
  bytes_total: number
  last_seen: string
  /** 该 IP 关联过的用户名；同一 IP 出现多个即值得注意 */
  users: string[]
}

export interface ImageStat {
  image_name: string
  registry: string
  category: string
  pull_count: number
  bytes_total: number
  unique_ips: number
}
```

在 `adminApi` 对象里、`ips` 之后加：

```ts
  ipsSuggest: (prefix: string) =>
    request<{ items: string[] }>(`/ips/suggest?${buildQuery({ prefix })}`),
```

- [ ] **Step 5: 构建验证**

Run: `cd web && npm run build`
Expected: 退出码 0。若报 `Property 'username' does not exist` 之类，说明 `PullSession` 接口没改到。

- [ ] **Step 6: 提交**

```bash
git add web/src/admin/api.ts web/src/lib/dateRange.ts
git commit -m "feat(web): support repeated query params and add IP suggestion client"
```

---

### Task 6: 前端 — 三个筛选组件

**Files:**
- Create: `web/src/components/ui/MultiSelect.vue`
- Create: `web/src/components/ui/Autocomplete.vue`
- Create: `web/src/components/ui/DateRange.vue`

**Interfaces:**
- Consumes: Task 5 的 `adminApi.ipsSuggest`、`dateRange.ts`
- Produces:
  - `MultiSelect`：`v-model` 绑定 `string[]`，props `{ options: SelectOption[]; placeholder?: string; class?: string }`
  - `Autocomplete`：`v-model` 绑定 `string`，props `{ fetch: (q: string) => Promise<string[]>; placeholder?: string; class?: string }`
  - `DateRange`：`v-model` 绑定 `{ preset: DateRangePreset; from: string; to: string }`

三个组件都**不得引入依赖**，并复用现有 `Select.vue` 的浮层做法（Teleport + 定位 + 点击外部关闭）。

- [ ] **Step 1: 写 MultiSelect.vue**

创建 `web/src/components/ui/MultiSelect.vue`。触发按钮的类名与 `Select.vue` 保持一致以保证观感统一：

```vue
<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref } from 'vue'
import type { HTMLAttributes } from 'vue'
import { Check, ChevronDown } from 'lucide-vue-next'
import { cn } from '@/lib/utils'
import type { SelectOption } from './Select.vue'

const model = defineModel<string[]>({ default: () => [] })

const props = withDefaults(
  defineProps<{
    options: SelectOption[]
    placeholder?: string
    class?: HTMLAttributes['class']
  }>(),
  { placeholder: '全部' },
)

const open = ref(false)
const root = ref<HTMLElement | null>(null)
const panel = ref<HTMLElement | null>(null)
const trigger = ref<HTMLElement | null>(null)
const panelStyle = ref<Record<string, string>>({})

const selectedLabels = computed(() =>
  props.options.filter((o) => model.value.includes(o.value)).map((o) => o.label),
)

function updatePosition() {
  const el = trigger.value
  if (!el) return
  const rect = el.getBoundingClientRect()
  const gap = 6
  const spaceBelow = window.innerHeight - rect.bottom - gap
  const spaceAbove = rect.top - gap
  const openUp = spaceBelow < 160 && spaceAbove > spaceBelow
  panelStyle.value = {
    position: 'fixed',
    left: `${Math.max(8, rect.left)}px`,
    width: `${Math.max(rect.width, 160)}px`,
    maxHeight: `${Math.min(280, openUp ? spaceAbove : spaceBelow)}px`,
    zIndex: '9999',
    ...(openUp
      ? { bottom: `${window.innerHeight - rect.top + gap}px`, top: 'auto' }
      : { top: `${rect.bottom + gap}px`, bottom: 'auto' }),
  }
}

async function toggle() {
  open.value = !open.value
  if (open.value) {
    await nextTick()
    updatePosition()
  }
}

function toggleValue(value: string) {
  const next = model.value.includes(value)
    ? model.value.filter((v) => v !== value)
    : [...model.value, value]
  model.value = next
}

function clear() {
  model.value = []
}

function onDocClick(e: MouseEvent) {
  const t = e.target as Node
  if (root.value?.contains(t) || panel.value?.contains(t)) return
  open.value = false
}

function onKey(e: KeyboardEvent) {
  if (e.key === 'Escape') open.value = false
}

function onReposition() {
  if (open.value) updatePosition()
}

onMounted(() => {
  document.addEventListener('click', onDocClick, true)
  document.addEventListener('keydown', onKey)
  window.addEventListener('resize', onReposition)
  window.addEventListener('scroll', onReposition, true)
})
onUnmounted(() => {
  document.removeEventListener('click', onDocClick, true)
  document.removeEventListener('keydown', onKey)
  window.removeEventListener('resize', onReposition)
  window.removeEventListener('scroll', onReposition, true)
})
</script>

<template>
  <div ref="root" :class="cn('relative', props.class)">
    <button
      ref="trigger"
      type="button"
      :aria-expanded="open"
      class="flex h-11 w-full items-center justify-between gap-2 rounded-xl border border-input bg-background/80 px-3.5 text-left text-sm outline-none transition-[border-color,box-shadow,background-color] duration-150 hover:bg-accent/40 focus-visible:border-ring focus-visible:ring-2 focus-visible:ring-ring/40"
      @click="toggle"
    >
      <span class="truncate" :class="selectedLabels.length ? 'text-foreground' : 'text-muted-foreground'">
        {{ selectedLabels.length ? selectedLabels.join('、') : placeholder }}
      </span>
      <ChevronDown
        class="size-4 shrink-0 text-muted-foreground transition-transform duration-200"
        :class="open ? 'rotate-180' : ''"
      />
    </button>

    <Teleport to="body">
      <Transition name="select-pop">
        <div
          v-if="open"
          ref="panel"
          :style="panelStyle"
          class="overflow-y-auto rounded-xl border border-border/80 bg-background p-1 shadow-xl shadow-black/10 dark:shadow-black/40"
        >
          <button
            type="button"
            class="mb-1 flex w-full items-center rounded-lg px-3 py-2 text-left text-sm text-muted-foreground transition-colors hover:bg-accent"
            @click="clear"
          >
            清空选择
          </button>
          <button
            v-for="opt in options"
            :key="opt.value"
            type="button"
            class="flex w-full items-center gap-2 rounded-lg px-3 py-2.5 text-left text-sm transition-colors"
            :class="model.includes(opt.value) ? 'bg-primary/10 text-primary' : 'text-foreground hover:bg-accent'"
            @click="toggleValue(opt.value)"
          >
            <span
              class="flex size-4 shrink-0 items-center justify-center rounded border"
              :class="model.includes(opt.value) ? 'border-primary bg-primary text-primary-foreground' : 'border-input'"
            >
              <Check v-if="model.includes(opt.value)" class="size-3" />
            </span>
            <span class="truncate">{{ opt.label }}</span>
          </button>
        </div>
      </Transition>
    </Teleport>
  </div>
</template>

<style scoped>
.select-pop-enter-active,
.select-pop-leave-active {
  transition: opacity 0.12s ease, transform 0.12s ease;
}
.select-pop-enter-from,
.select-pop-leave-to {
  opacity: 0;
  transform: translateY(-4px) scale(0.98);
}
</style>
```

- [ ] **Step 2: 写 Autocomplete.vue**

创建 `web/src/components/ui/Autocomplete.vue`。要点：输入防抖 200ms、只在前缀非空时请求、选中建议后关闭、点击外部关闭。

```vue
<script setup lang="ts">
import { nextTick, onUnmounted, ref, watch } from 'vue'
import type { HTMLAttributes } from 'vue'
import { cn } from '@/lib/utils'

const model = defineModel<string>({ default: '' })

const props = withDefaults(
  defineProps<{
    /** 由调用方提供取数逻辑，组件本身不关心来源 */
    fetch: (q: string) => Promise<string[]>
    placeholder?: string
    class?: HTMLAttributes['class']
  }>(),
  { placeholder: '' },
)

const suggestions = ref<string[]>([])
const open = ref(false)
const active = ref(0)
let timer: ReturnType<typeof setTimeout> | undefined
let seq = 0

function schedule(q: string) {
  if (timer) clearTimeout(timer)
  // 前缀为空时不去请求——后端也会拒绝，这里省一次往返
  if (!q.trim()) {
    suggestions.value = []
    open.value = false
    return
  }
  timer = setTimeout(async () => {
    const mine = ++seq
    try {
      const items = await props.fetch(q.trim())
      // 丢弃过期响应，避免慢请求覆盖新结果
      if (mine !== seq) return
      suggestions.value = items
      active.value = 0
      open.value = items.length > 0
    } catch {
      if (mine !== seq) return
      suggestions.value = []
      open.value = false
    }
  }, 200)
}

watch(model, (v) => schedule(v))

async function pick(v: string) {
  model.value = v
  open.value = false
  suggestions.value = []
  await nextTick()
}

function onKeydown(e: KeyboardEvent) {
  if (!open.value) return
  if (e.key === 'ArrowDown') {
    e.preventDefault()
    active.value = Math.min(active.value + 1, suggestions.value.length - 1)
  } else if (e.key === 'ArrowUp') {
    e.preventDefault()
    active.value = Math.max(active.value - 1, 0)
  } else if (e.key === 'Enter') {
    e.preventDefault()
    const v = suggestions.value[active.value]
    if (v) void pick(v)
  } else if (e.key === 'Escape') {
    open.value = false
  }
}

function onBlur() {
  // 延迟关闭，否则点击建议项时 blur 先触发、click 落空
  setTimeout(() => {
    open.value = false
  }, 150)
}

onUnmounted(() => {
  if (timer) clearTimeout(timer)
})
</script>

<template>
  <div :class="cn('relative', props.class)">
    <input
      v-model="model"
      type="text"
      :placeholder="placeholder"
      class="flex h-11 w-full rounded-xl border border-input bg-background/80 px-3.5 text-sm outline-none transition-[border-color,box-shadow] duration-150 focus-visible:border-ring focus-visible:ring-2 focus-visible:ring-ring/40"
      @keydown="onKeydown"
      @blur="onBlur"
      @focus="open = suggestions.length > 0"
    />
    <div
      v-if="open"
      class="absolute inset-x-0 top-full z-50 mt-1.5 max-h-64 overflow-y-auto rounded-xl border border-border/80 bg-background p-1 shadow-xl shadow-black/10 dark:shadow-black/40"
    >
      <button
        v-for="(s, i) in suggestions"
        :key="s"
        type="button"
        class="block w-full rounded-lg px-3 py-2 text-left font-mono text-xs transition-colors"
        :class="i === active ? 'bg-primary/10 text-primary' : 'text-foreground hover:bg-accent'"
        @mousedown.prevent="pick(s)"
      >
        {{ s }}
      </button>
    </div>
  </div>
</template>
```

- [ ] **Step 3: 写 DateRange.vue**

创建 `web/src/components/ui/DateRange.vue`：

```vue
<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { HTMLAttributes } from 'vue'
import Input from './Input.vue'
import { cn } from '@/lib/utils'
import { dateInputToRFC3339, type DateRangePreset } from '@/lib/dateRange'

export type DateRangeValue = { preset: DateRangePreset; from: string; to: string }

const model = defineModel<DateRangeValue>({
  default: () => ({ preset: 'all', from: '', to: '' }),
})

const props = defineProps<{ class?: HTMLAttributes['class'] }>()

const presets: Array<{ value: DateRangePreset; label: string }> = [
  { value: 'today', label: '今天' },
  { value: '7d', label: '近 7 天' },
  { value: '30d', label: '近 30 天' },
  { value: 'all', label: '全部' },
  { value: 'custom', label: '自定义' },
]

// 自定义模式下用本地日期串驱动两个 date input
const fromDate = ref('')
const toDate = ref('')

const isCustom = computed(() => model.value.preset === 'custom')

watch(fromDate, () => {
  model.value = { ...model.value, from: dateInputToRFC3339(fromDate.value, false) }
})
watch(toDate, () => {
  model.value = { ...model.value, to: dateInputToRFC3339(toDate.value, true) }
})

function pick(preset: DateRangePreset) {
  if (preset === 'custom') {
    model.value = { preset, from: dateInputToRFC3339(fromDate.value, false), to: dateInputToRFC3339(toDate.value, true) }
    return
  }
  // from/to 由父组件调用 presetRange 计算，这里只切换预设，避免组件间职责重叠
  model.value = { preset, from: '', to: '' }
}
</script>

<template>
  <div :class="cn('space-y-2', props.class)">
    <div class="flex flex-wrap gap-1.5">
      <button
        v-for="p in presets"
        :key="p.value"
        type="button"
        class="rounded-lg border px-3 py-1.5 text-xs transition-colors"
        :class="
          model.preset === p.value
            ? 'border-primary bg-primary/10 text-primary'
            : 'border-input text-muted-foreground hover:bg-accent'
        "
        @click="pick(p.value)"
      >
        {{ p.label }}
      </button>
    </div>
    <div v-if="isCustom" class="flex items-center gap-2">
      <Input v-model="fromDate" type="date" class="h-10" />
      <span class="shrink-0 text-xs text-muted-foreground">至</span>
      <Input v-model="toDate" type="date" class="h-10" />
    </div>
  </div>
</template>
```

**关于 `from`/`to` 的计算**：`presetRange()` 由**父组件**在发起查询时调用（它需要「现在」这个时刻）。这样 `DateRange.vue` 只负责「用户选了哪个预设」，不掺和具体时刻换算，职责单一。

- [ ] **Step 4: 构建验证**

Run: `cd web && npm run build`
Expected: 退出码 0。`MultiSelect` 从 `./Select.vue` 导入 `SelectOption` 类型——若 `Select.vue` 未用 `export type` 导出它，构建会报错，此时在 `Select.vue` 里确认该类型是 `export type SelectOption`（它已经是）。

- [ ] **Step 5: 提交**

```bash
git add web/src/components/ui/MultiSelect.vue web/src/components/ui/Autocomplete.vue web/src/components/ui/DateRange.vue
git commit -m "feat(web): add multiselect, autocomplete and date-range filter controls"
```

---

### Task 7: 前端 — 全局拉取页重构

**Files:**
- Modify: `web/src/admin/pages/PullsPage.vue`

**Interfaces:**
- Consumes: Task 5 的 `buildQuery`/`PullSession.username`/`IPStat`、Task 6 的三个组件、`presetRange`
- Produces: 无（叶子页面）

- [ ] **Step 1: 重写脚本段**

把 `PullsPage.vue` 的 `<script setup>` 整体替换为：

```vue
<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import Button from '@/components/ui/Button.vue'
import Input from '@/components/ui/Input.vue'
import Select from '@/components/ui/Select.vue'
import MultiSelect from '@/components/ui/MultiSelect.vue'
import Autocomplete from '@/components/ui/Autocomplete.vue'
import DateRange from '@/components/ui/DateRange.vue'
import Card from '@/components/ui/Card.vue'
import CardContent from '@/components/ui/CardContent.vue'
import Badge from '@/components/ui/Badge.vue'
import DataTable from '@/components/ui/DataTable.vue'
import { presetRange, type DateRangePreset } from '@/lib/dateRange'
import {
  adminApi,
  displayPullName,
  formatBytes,
  formatTime,
  type PullSession,
  type User,
} from '../api'

// 来源与类型是两个不同维度：registry 是仓库来源，category 是镜像归属
const registryOptions = [
  { value: '', label: '全部来源' },
  { value: 'docker.io', label: 'docker.io' },
  { value: 'ghcr.io', label: 'ghcr.io' },
  { value: 'gcr.io', label: 'gcr.io' },
  { value: 'quay.io', label: 'quay.io' },
  { value: 'registry.k8s.io', label: 'registry.k8s.io' },
  { value: 'registry.gitlab.com', label: 'registry.gitlab.com' },
]
const categoryOptions = [
  { value: '', label: '全部类型' },
  { value: 'library', label: 'library' },
  { value: 'user', label: 'user' },
]

const route = useRoute()
const items = ref<PullSession[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = 20
const loading = ref(false)
const selected = ref<{ session: PullSession; events: any[] } | null>(null)

const ip = ref(String(route.query.ip || ''))
const image = ref('')
const registry = ref('')
const category = ref('')
const userIds = ref<string[]>([])
const userOptions = ref<{ value: string; label: string }[]>([])
const dateRange = ref<{ preset: DateRangePreset; from: string; to: string }>({
  preset: 'all',
  from: '',
  to: '',
})

async function loadUsers() {
  try {
    const res = await adminApi.users()
    userOptions.value = res.items.map((u: User) => ({
      value: String(u.id),
      label: u.username,
    }))
  } catch {
    // 拉不到用户列表时筛选框为空，表格仍可用（只是没有名字可显示）
  }
}

async function load() {
  loading.value = true
  try {
    const range = presetRange(dateRange.value.preset)
    const res = await adminApi.pulls({
      page: page.value,
      page_size: pageSize,
      ip: ip.value,
      image: image.value,
      registry: registry.value,
      category: category.value,
      user_id: userIds.value,
      from: dateRange.value.preset === 'custom' ? dateRange.value.from : range.from,
      to: dateRange.value.preset === 'custom' ? dateRange.value.to : range.to,
    })
    items.value = res.items
    total.value = res.total
  } finally {
    loading.value = false
  }
}

function search() {
  page.value = 1
  load()
}

async function openDetail(id: string) {
  selected.value = await adminApi.pull(id)
}

onMounted(() => {
  loadUsers()
  load()
})
watch(page, load)
</script>
```

- [ ] **Step 2: 重写筛选栏**

把模板里第一个 `<Card>`（筛选栏）替换为：

```html
    <Card>
      <CardContent class="grid gap-3 pt-5 md:grid-cols-3">
        <Autocomplete
          v-model="ip"
          :fetch="(q) => adminApi.ipsSuggest(q).then((r) => r.items)"
          placeholder="按 IP 筛选（输入前几位会有建议）"
        />
        <Input v-model="image" placeholder="按镜像名称筛选" />
        <MultiSelect v-model="userIds" :options="userOptions" placeholder="全部用户" />
        <Select v-model="registry" :options="registryOptions" />
        <Select v-model="category" :options="categoryOptions" />
        <Button class="rounded-xl" @click="search">查询</Button>
        <DateRange v-model="dateRange" class="md:col-span-3" @update:model-value="search" />
      </CardContent>
    </Card>
```

- [ ] **Step 3: 改表格列**

把 `<template #head>` 的 `<tr>` 替换为（**去掉「层/请求」，加上「用户」**）：

```html
            <tr>
              <th class="px-3 py-2.5 font-medium whitespace-nowrap">开始时间</th>
              <th class="px-3 py-2.5 font-medium">内容</th>
              <th class="px-3 py-2.5 font-medium whitespace-nowrap">用户</th>
              <th class="px-3 py-2.5 font-medium whitespace-nowrap">IP</th>
              <th class="px-3 py-2.5 font-medium whitespace-nowrap">类型</th>
              <th class="px-3 py-2.5 font-medium whitespace-nowrap text-right">流量</th>
              <th class="px-3 py-2.5 font-medium whitespace-nowrap"></th>
            </tr>
```

把数据行 `<tr v-for="p in items" ...>` 整段替换为：

```html
          <tr
            v-for="p in items"
            :key="p.id"
            class="border-t border-border/70 transition-colors hover:bg-accent/40"
          >
            <td class="px-3 py-2.5 tabular-nums whitespace-nowrap">{{ formatTime(p.started_at) }}</td>
            <td class="max-w-[14rem] px-3 py-2.5">
              <div class="truncate font-medium" :title="p.image_name">{{ displayPullName(p) }}</div>
              <div class="truncate text-xs text-muted-foreground">{{ p.registry }}</div>
            </td>
            <td class="px-3 py-2.5 whitespace-nowrap">
              <span v-if="p.username">{{ p.username }}</span>
              <span v-else-if="p.user_id" class="text-muted-foreground">#{{ p.user_id }}</span>
              <span v-else class="text-muted-foreground">匿名</span>
            </td>
            <td class="px-3 py-2.5 font-mono text-xs whitespace-nowrap">{{ p.client_ip }}</td>
            <td class="px-3 py-2.5 whitespace-nowrap"><Badge variant="secondary">{{ p.category }}</Badge></td>
            <td class="px-3 py-2.5 text-right tabular-nums whitespace-nowrap">{{ formatBytes(p.bytes_total) }}</td>
            <td class="px-3 py-2.5 whitespace-nowrap">
              <Button size="sm" variant="ghost" @click="openDetail(p.id)">详情</Button>
            </td>
          </tr>
```

**注意两处语义修正**：

- 该列的表头是**类型**而不是「来源」——它显示的是 `p.category`（library/user），与筛选栏里「来源=registry、类型=category」的命名保持一套口径。
- 因此本页不再需要 `pullSourceLabel`（它返回的是 registry，而 registry 已经显示在镜像名下方那一行）。Step 1 的 import 里已经不含它，**不要**再把它加回来——`noUnusedLocals` 会让构建失败。

- [ ] **Step 4: 空态与加载态**

把 `DataTable` 之后的：

```html
        <p v-if="!items.length && !loading" class="py-6 text-center text-sm text-muted-foreground">暂无记录</p>
```

替换为：

```html
        <p v-if="loading" class="py-6 text-center text-sm text-muted-foreground">加载中…</p>
        <p v-else-if="!items.length" class="py-6 text-center text-sm text-muted-foreground">
          没有符合条件的记录，试试放宽筛选条件
        </p>
```

- [ ] **Step 5: 详情弹窗补用户**

把详情弹窗里的 `<div class="grid grid-cols-2 gap-2 text-sm">` 内，`<div>HTTP 请求：{{ selected.session.request_count }}</div>` 替换为：

```html
            <div>
              用户：{{ selected.session.username || (selected.session.user_id ? '#' + selected.session.user_id : '匿名') }}
            </div>
```

把详情弹窗里的：

```html
            <div>类别：<Badge variant="secondary">{{ pullSourceLabel(selected.session) }}</Badge></div>
```

替换为：

```html
            <div>类别：<Badge variant="secondary">{{ selected.session.category }}</Badge></div>
```

（`request_count` 仍在 API 与数据库里，只是不再展示；这一处替换是因为详情区本就没有「HTTP 请求」以外的位置放用户。）

**这两处必须同改**：Step 1 已经移除了 `pullSourceLabel` 的 import 并禁止再加回来，而详情弹窗的「类别」行也在调用它。只改表格列、漏掉弹窗那一行的话，构建会以 `TS2339`（找不到该符号）失败——这是原计划的疏漏。弹窗里 registry 已显示在标题行（`{{ selected.session.registry }} · {{ selected.session.client_ip }}`），所以「类别」行改为直接显示 `category` 不会丢信息。

- [ ] **Step 6: 构建验证**

Run: `cd web && npm run build`
Expected: 退出码 0。若报 `'User' is not exported`，确认 `web/src/admin/api.ts` 里有 `export interface User`。

- [ ] **Step 7: 提交**

```bash
git add web/src/admin/pages/PullsPage.vue
git commit -m "feat(web): rework pulls page filters, user column and table polish"
```

---

### Task 8: 前端 — IP 分析页重构

**Files:**
- Modify: `web/src/admin/pages/IPsPage.vue`

**Interfaces:**
- Consumes: Task 4 的 `ListIPStats` 时间范围、Task 5 的 `IPStat`/`adminApi.ipsSuggest`、Task 6 的 `Autocomplete`/`DateRange`、`presetRange`
- Produces: 无

- [ ] **Step 1: 替换脚本段**

```vue
<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import Button from '@/components/ui/Button.vue'
import Autocomplete from '@/components/ui/Autocomplete.vue'
import DateRange from '@/components/ui/DateRange.vue'
import Badge from '@/components/ui/Badge.vue'
import Card from '@/components/ui/Card.vue'
import CardContent from '@/components/ui/CardContent.vue'
import DataTable from '@/components/ui/DataTable.vue'
import { presetRange, type DateRangePreset } from '@/lib/dateRange'
import { adminApi, formatBytes, formatTime, type IPStat } from '../api'
import { useAuth } from '../auth'
import { useRouter } from 'vue-router'
import { toastError, toastSuccess } from '@/lib/toast'

const items = ref<IPStat[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = 20
const ip = ref('')
const loading = ref(false)
const dateRange = ref<{ preset: DateRangePreset; from: string; to: string }>({
  preset: 'all',
  from: '',
  to: '',
})
const { isAdmin } = useAuth()
const router = useRouter()

async function load() {
  loading.value = true
  try {
    const range = presetRange(dateRange.value.preset)
    const res = await adminApi.ips({
      page: page.value,
      page_size: pageSize,
      ip: ip.value,
      from: dateRange.value.preset === 'custom' ? dateRange.value.from : range.from,
      to: dateRange.value.preset === 'custom' ? dateRange.value.to : range.to,
    })
    items.value = res.items
    total.value = res.total
  } finally {
    loading.value = false
  }
}

function search() {
  page.value = 1
  load()
}

async function ban(ipAddr: string) {
  if (!isAdmin.value) return
  try {
    await adminApi.addBlackIP(ipAddr)
    toastSuccess(`已拉黑 ${ipAddr}`)
  } catch (e: any) {
    toastError(e?.message || '操作失败')
  }
}

onMounted(load)
watch(page, load)
</script>
```

- [ ] **Step 2: 替换筛选栏**

```html
    <Card>
      <CardContent class="flex flex-col gap-3 pt-5">
        <div class="flex flex-col gap-3 sm:flex-row">
          <Autocomplete
            v-model="ip"
            :fetch="(q) => adminApi.ipsSuggest(q).then((r) => r.items)"
            placeholder="按 IP 筛选（输入前几位会有建议）"
            class="sm:max-w-xs"
          />
          <Button class="rounded-xl" @click="search">查询</Button>
        </div>
        <DateRange v-model="dateRange" @update:model-value="search" />
      </CardContent>
    </Card>
```

- [ ] **Step 3: 加用户列**

表头 `<tr>` 中，在「IP」之后插入一列：

```html
              <th class="px-3 py-2.5 font-medium whitespace-nowrap">用户</th>
```

数据行里在 IP 单元格之后插入：

```html
            <td class="px-3 py-2.5 whitespace-nowrap">
              <template v-if="it.users.length">
                <span v-if="it.users.length > 1" class="mr-1.5 inline-block" title="多个用户共用同一 IP，值得留意">
                  <Badge variant="destructive">{{ it.users.length }}</Badge>
                </span>
                <span :class="it.users.length > 1 ? 'text-destructive' : ''">{{ it.users.join('、') }}</span>
              </template>
              <span v-else class="text-muted-foreground">匿名</span>
            </td>
```

- [ ] **Step 4: 数字列右对齐与空态/加载态**

「拉取次数」与「总流量」两个 `<th>` 加 `text-right`，对应 `<td>` 加 `text-right tabular-nums`（`pull_count` 与 `formatBytes(...)` 两处）。行 `<tr>` 加 `transition-colors hover:bg-accent/40`。

把：

```html
        <p v-if="!items.length" class="py-6 text-center text-sm text-muted-foreground">暂无数据</p>
```

替换为：

```html
        <p v-if="loading" class="py-6 text-center text-sm text-muted-foreground">加载中…</p>
        <p v-else-if="!items.length" class="py-6 text-center text-sm text-muted-foreground">
          没有符合条件的记录
        </p>
```

- [ ] **Step 5: 构建验证**

Run: `cd web && npm run build`
Expected: 退出码 0。

- [ ] **Step 6: 提交**

```bash
git add web/src/admin/pages/IPsPage.vue
git commit -m "feat(web): show per-IP users and add IP autocomplete on the IPs page"
```

---

### Task 9: 前端 — 镜像统计页重构

**Files:**
- Modify: `web/src/admin/pages/ImagesPage.vue`

**Interfaces:**
- Consumes: Task 4 的 `from`/`to` 支持、Task 5 的 `buildQuery`、Task 6 的 `DateRange`、`presetRange`
- Produces: 无

- [ ] **Step 1: 替换脚本段**

```vue
<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import Button from '@/components/ui/Button.vue'
import Input from '@/components/ui/Input.vue'
import Select from '@/components/ui/Select.vue'
import DateRange from '@/components/ui/DateRange.vue'
import Card from '@/components/ui/Card.vue'
import CardContent from '@/components/ui/CardContent.vue'
import Badge from '@/components/ui/Badge.vue'
import DataTable from '@/components/ui/DataTable.vue'
import { presetRange, type DateRangePreset } from '@/lib/dateRange'
import { adminApi, formatBytes, type ImageStat } from '../api'

const registryOptions = [
  { value: '', label: '全部来源' },
  { value: 'docker.io', label: 'docker.io' },
  { value: 'ghcr.io', label: 'ghcr.io' },
  { value: 'gcr.io', label: 'gcr.io' },
  { value: 'quay.io', label: 'quay.io' },
  { value: 'registry.k8s.io', label: 'registry.k8s.io' },
  { value: 'registry.gitlab.com', label: 'registry.gitlab.com' },
]
const categoryOptions = [
  { value: '', label: '全部类型' },
  { value: 'library', label: 'library' },
  { value: 'user', label: 'user' },
]

const items = ref<ImageStat[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = 20
const image = ref('')
const category = ref('')
const registry = ref('')
const loading = ref(false)
const dateRange = ref<{ preset: DateRangePreset; from: string; to: string }>({
  preset: 'all',
  from: '',
  to: '',
})

async function load() {
  loading.value = true
  try {
    const range = presetRange(dateRange.value.preset)
    const res = await adminApi.images({
      page: page.value,
      page_size: pageSize,
      image: image.value,
      category: category.value,
      registry: registry.value,
      from: dateRange.value.preset === 'custom' ? dateRange.value.from : range.from,
      to: dateRange.value.preset === 'custom' ? dateRange.value.to : range.to,
    })
    items.value = res.items
    total.value = res.total
  } finally {
    loading.value = false
  }
}

function search() {
  page.value = 1
  load()
}

onMounted(load)
watch(page, load)
</script>
```

`ImageStat` 由 Task 5 的 `web/src/admin/api.ts` 提供，本任务直接用即可。

- [ ] **Step 2: 替换筛选栏**

```html
    <Card>
      <CardContent class="grid gap-3 pt-5 md:grid-cols-4">
        <Input v-model="image" placeholder="镜像名称" />
        <Select v-model="registry" :options="registryOptions" />
        <Select v-model="category" :options="categoryOptions" />
        <Button class="rounded-xl" @click="search">查询</Button>
        <DateRange v-model="dateRange" class="md:col-span-4" @update:model-value="search" />
      </CardContent>
    </Card>
```

- [ ] **Step 3: 表格对齐与状态**

表头「拉取次数」「独立 IP」「总流量」三个 `<th>` 加 `text-right`；对应 `<td>` 加 `text-right tabular-nums`。行 `<tr>` 加 `transition-colors hover:bg-accent/40`。

**修正一处既有错误**：类别列的单元格当前是 `{{ pullSourceLabel(it) }}`，而 `pullSourceLabel` 返回 `p.registry || 'Docker'` ——也就是说它显示的是 **registry**，与同表已有的 Registry 列内容重复，类别列从来没显示过真正的类别。改为直接显示 `it.category`：

```html
            <td class="px-3 py-2.5 whitespace-nowrap"><Badge variant="secondary">{{ it.category }}</Badge></td>
```

`pullSourceLabel` 在 `web/src/admin/api.ts` 里仍被 DashboardPage / UserPullsPage / UserDashboardPage 使用，**不要删除它**，只是本页不再引用。

把空态那行替换为：

```html
        <p v-if="loading" class="py-6 text-center text-sm text-muted-foreground">加载中…</p>
        <p v-else-if="!items.length" class="py-6 text-center text-sm text-muted-foreground">
          没有符合条件的记录
        </p>
```

- [ ] **Step 4: 构建验证**

Run: `cd web && npm run build`
Expected: 退出码 0。

- [ ] **Step 5: 提交**

```bash
git add web/src/admin/pages/ImagesPage.vue web/src/admin/api.ts
git commit -m "feat(web): add time range and registry/category filters to image stats"
```

---

### Task 10: 端到端验证

**Files:** 无改动（纯验证）

**Interfaces:**
- Consumes: Task 1–9 的全部产物
- Produces: 一份可复现的验证结论

- [ ] **Step 1: 全量测试与构建**

```bash
cd e:/Programming/HTML/hubproxy/src && go test -count=1 ./...
cd ../web && npm run build
```

Expected: Go 全绿（基线 0 失败），前端构建退出码 0。

- [ ] **Step 2: 确认没有遗留的旧签名**

```bash
cd e:/Programming/HTML/hubproxy
grep -rn "UserID:" src --include=*.go
grep -rn "Record<string, string | number | undefined>" web/src
grep -rn "层/请求" web/src
```

Expected：三条都无输出。第一条若命中，说明还有地方在用旧的单值字段名；第三条若命中，「层/请求」列没删干净。

- [ ] **Step 3: 确认 NULL user_id 未被 JOIN 吞掉**

```bash
cd e:/Programming/HTML/hubproxy
grep -rn "JOIN users" src/db/stats.go
```

Expected：`ListPullSessions` 里那一处必须是 `LEFT JOIN users`。若是 `JOIN`，匿名行会整条消失——这是本计划最容易搞错的地方。

- [ ] **Step 4: 启动服务并验证接口**

```bash
cd e:/Programming/HTML/hubproxy/src && CONFIG_PATH=./config.toml go run . &
sleep 3
curl -s "http://127.0.0.1:5000/api/admin/pulls?user_id=1&user_id=2" | head -c 300
curl -s "http://127.0.0.1:5000/api/admin/ips/suggest?prefix=1" | head -c 200
```

Expected：第一条返回 JSON（未登录时返回 401 `UNAUTHORIZED` 也算通过——说明路由存在）；第二条同样。**验证完必须关闭后台进程**并确认端口已释放。

- [ ] **Step 5: 人工浏览器确认（实施者无法执行）**

以下必须在浏览器中由人工确认，实施者**不得声称已完成**：

1. 全局拉取页：IP 输入框输入 `1` 是否出现前缀建议
2. 用户多选：选中两个用户后查询，结果是否只剩这两人的记录
3. 时间段：切到「近 7 天」再切「自定义」，起止日期是否正确生效
4. 表格用户列：有令牌的行显示用户名，无令牌的行显示「匿名」
5. 表格已看不到「层/请求」列
6. IP 分析页：用户列正确显示；**同一 IP 有多个用户时应出现红色标记**
7. 镜像统计页：时间段筛选生效，且**没有**用户列
8. IP 自动补全的下拉面板**不被容器裁切**：`Autocomplete.vue` 没有用 Teleport（与 `Select.vue` / `MultiSelect.vue` 不同），面板是 `absolute` 定位在内层 `relative` 里。当前筛选栏所在的 `Card` / `CardContent` 只有 `p-5`、没有 overflow 限制，所以预期不会被裁——但这条只有肉眼能确认。**若发现被裁，把 `Autocomplete.vue` 改成与 `MultiSelect.vue` 相同的 Teleport + fixed 定位方案。**
9. 选中一条 IP 建议后，下拉面板**不应自己弹回来**（这是 Task 6 修掉的两个 bug 之一：`pick()` 写 model 触发 watcher 重新取数）。输入清空后同样不应弹回旧建议

- [ ] **Step 6: 收尾**

```bash
cd e:/Programming/HTML/hubproxy && git status --short
```

Expected: 工作区干净（`src/data/` 被 gitignore，不算脏）。
