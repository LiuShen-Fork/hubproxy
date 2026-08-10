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
