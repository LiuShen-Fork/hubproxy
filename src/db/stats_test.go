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
