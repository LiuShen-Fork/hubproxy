package db

import "testing"

func TestDigestManifestStaysInExistingPullSession(t *testing.T) {
	useTestDB(t)
	if err := migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	now := Now()
	_, err := DB.Exec(
		"INSERT INTO pull_sessions "+
			"(id, client_ip, image_name, registry, tag, category, started_at, last_seen_at, status, "+
			"bytes_total, layer_count, request_count) "+
			"VALUES ('s1', '192.0.2.10', 'thomiceli/opengist', 'docker.io', '1.15.0', 'user', "+
			"?, ?, 'active', 40700000, 4, 8)",
		now, now,
	)
	if err != nil {
		t.Fatalf("insert active session: %v", err)
	}

	sess, created, err := FindOrCreatePullSession(
		"192.0.2.10",
		"thomiceli/opengist",
		"docker.io",
		"digest",
		"manifests",
		0,
	)
	if err != nil {
		t.Fatalf("find or create digest manifest: %v", err)
	}
	if created {
		t.Fatal("digest manifest created a second pull session")
	}
	if sess == nil || sess.ID != "s1" {
		t.Fatalf("digest manifest attached to wrong session: %#v", sess)
	}

	var total int
	if err := DB.QueryRow("SELECT COUNT(*) FROM pull_sessions").Scan(&total); err != nil {
		t.Fatalf("count sessions: %v", err)
	}
	if total != 1 {
		t.Fatalf("pull session count = %d, want 1", total)
	}
}

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

	list, _, err := ListIPStats("", "", "", 1, 50)
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
	if byIP["10.0.0.8"].Users == nil {
		t.Fatal("纯匿名的 IP 的 Users 必须是空切片而不是 nil——nil 会被序列化成 null，前端读 .length 会抛 TypeError")
	}
	if len(byIP["10.0.0.8"].Users) != 0 {
		t.Fatalf("纯匿名的 IP 不应有用户，实际 %#v", byIP["10.0.0.8"].Users)
	}
}

func TestListPullSessionsCountedOnlyExcludesZeroLayerProbes(t *testing.T) {
	useTestDB(t)
	if err := migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// 正常计数的一次拉取（helper 固定 layer_count = 1）
	insertPullSession(t, "s-counted", "10.0.0.1", "alpine", 0)
	// 只看过 manifest、没下过任何层的探测记录：layer_count = 0，不该算一次拉取。
	// helper 把 layer_count 写死成 1，所以这里直接插入以便控制该列。
	now := Now()
	if _, err := DB.Exec(
		`INSERT INTO pull_sessions
		 (id, client_ip, image_name, registry, tag, category, started_at, last_seen_at,
		  status, bytes_total, layer_count, request_count)
		 VALUES ('s-probe', '10.0.0.2', 'busybox', 'docker.io', 'latest', 'library',
		         ?, ?, 'active', 0, 0, 1)`,
		now, now,
	); err != nil {
		t.Fatalf("insert probe session: %v", err)
	}

	got, total, err := ListPullSessions(PullListFilter{Page: 1, PageSize: 10, CountedOnly: true})
	if err != nil {
		t.Fatalf("CountedOnly 查询不应报错：%v", err)
	}
	if total != 1 || len(got) != 1 {
		t.Fatalf("CountedOnly 应只返回已计数的 1 条，实际 total=%d len=%d", total, len(got))
	}
	if got[0].ID != "s-counted" {
		t.Fatalf("CountedOnly 返回了未计数的探测记录 %q", got[0].ID)
	}
}

func TestGetPullSessionResolvesUsernameAndKeepsAnonymous(t *testing.T) {
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

	attributed, _, err := GetPullSession("s-alice")
	if err != nil {
		t.Fatalf("get attributed session: %v", err)
	}
	if attributed.Username != "alice" {
		t.Fatalf("详情接口的用户名应为 alice，实际 %q", attributed.Username)
	}

	anonymous, _, err := GetPullSession("s-anon")
	if err != nil {
		t.Fatalf("匿名行的详情不应报错：%v", err)
	}
	if anonymous.Username != "" {
		t.Fatalf("匿名行的用户名应为空，实际 %q", anonymous.Username)
	}
}

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

func TestListIPStatsTimeRangeAlsoNarrowsUsers(t *testing.T) {
	useTestDB(t)
	if err := migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	now := Now()
	for _, u := range []struct {
		id   int64
		name string
	}{{1, "earlyuser"}, {2, "lateuser"}} {
		if _, err := DB.Exec(
			`INSERT INTO users (id, username, password_hash, role, created_at, updated_at)
			 VALUES (?, ?, 'hash', 'user', ?, ?)`, u.id, u.name, now, now,
		); err != nil {
			t.Fatalf("insert user: %v", err)
		}
	}
	insert := func(id string, userID int64, startedAt string) {
		t.Helper()
		if _, err := DB.Exec(
			`INSERT INTO pull_sessions
			 (id, client_ip, image_name, registry, tag, category, started_at, last_seen_at,
			  status, bytes_total, layer_count, request_count, user_id)
			 VALUES (?, '10.7.7.7', 'app', 'docker.io', 'latest', 'user', ?, ?, 'completed', 10, 1, 1, ?)`,
			id, startedAt, startedAt, userID,
		); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}
	// 同一个 IP，两个用户，时间相差 8 个月
	insert("s-early", 1, "2026-01-01T00:00:00Z")
	insert("s-late", 2, "2026-09-01T00:00:00Z")

	// 不传时间范围：两个用户都应出现
	all, _, err := ListIPStats("10.7.7.7", "", "", 1, 50)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(all) != 1 || len(all[0].Users) != 2 {
		t.Fatalf("不传时间范围应关联两个用户，实际 %#v", all)
	}

	// 只取 6 月之后：聚合行与 users 列都必须只剩 lateuser，
	// 否则这一行的 pull_count 与 users 描述的是两个不同的时间范围。
	win, total, err := ListIPStats("10.7.7.7", "2026-06-01T00:00:00Z", "", 1, 50)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 1 || len(win) != 1 {
		t.Fatalf("时间窗内应只剩 1 行，实际 total=%d %#v", total, win)
	}
	if len(win[0].Users) != 1 || win[0].Users[0] != "lateuser" {
		t.Fatalf("users 列必须与时间窗一致，应只有 lateuser，实际 %#v", win[0].Users)
	}
}
