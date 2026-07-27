package db

import "testing"

func TestMigrationDetachesPullRecordsFromTokens(t *testing.T) {
	useTestDB(t)

	legacySchema := `
CREATE TABLE users (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	username TEXT NOT NULL UNIQUE COLLATE NOCASE,
	password_hash TEXT NOT NULL,
	role TEXT NOT NULL DEFAULT 'user',
	must_change_password INTEGER NOT NULL DEFAULT 0,
	daily_pull_limit INTEGER NOT NULL DEFAULT 30,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	last_login_at TEXT
);
CREATE TABLE user_access_tokens (
	token TEXT PRIMARY KEY,
	user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	status TEXT NOT NULL DEFAULT 'active',
	created_at TEXT NOT NULL,
	revoked_at TEXT
);
CREATE TABLE pull_sessions (
	id TEXT PRIMARY KEY,
	client_ip TEXT NOT NULL,
	image_name TEXT NOT NULL,
	registry TEXT NOT NULL DEFAULT 'docker.io',
	tag TEXT NOT NULL DEFAULT 'latest',
	category TEXT NOT NULL DEFAULT 'library',
	started_at TEXT NOT NULL,
	last_seen_at TEXT NOT NULL,
	completed_at TEXT,
	status TEXT NOT NULL DEFAULT 'active',
	bytes_total INTEGER NOT NULL DEFAULT 0,
	layer_count INTEGER NOT NULL DEFAULT 0,
	request_count INTEGER NOT NULL DEFAULT 0,
	user_id INTEGER,
	access_token TEXT
);
CREATE INDEX idx_pull_sessions_token ON pull_sessions(access_token);
`
	if _, err := DB.Exec(legacySchema); err != nil {
		t.Fatalf("create legacy schema: %v", err)
	}
	now := Now()
	if _, err := DB.Exec(
		`INSERT INTO users (id, username, password_hash, role, created_at, updated_at) VALUES (7, 'legacy', 'hash', 'user', ?, ?)`,
		now, now,
	); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if _, err := DB.Exec(
		`INSERT INTO user_access_tokens (token, user_id, status, created_at) VALUES ('Ab12Cd34', 7, 'revoked', ?)`,
		now,
	); err != nil {
		t.Fatalf("insert token: %v", err)
	}
	if _, err := DB.Exec(
		`INSERT INTO pull_sessions
		 (id, client_ip, image_name, registry, tag, category, started_at, last_seen_at, status,
		  bytes_total, layer_count, request_count, user_id, access_token)
		 VALUES ('pull-1', '192.0.2.1', 'owner/image', 'ghcr.io', 'v1', 'user', ?, ?, 'completed', 12345, 2, 4, NULL, 'Ab12Cd34')`,
		now, now,
	); err != nil {
		t.Fatalf("insert pull: %v", err)
	}

	if err := migrate(); err != nil {
		t.Fatalf("migrate legacy database: %v", err)
	}

	cols, err := tableColumns("pull_sessions")
	if err != nil {
		t.Fatalf("read pull columns: %v", err)
	}
	if cols["access_token"] {
		t.Fatal("access_token column still exists after migration")
	}

	var userID, bytesTotal, layerCount, requestCount int64
	var imageName, registry, tag string
	if err := DB.QueryRow(
		`SELECT user_id, image_name, registry, tag, bytes_total, layer_count, request_count FROM pull_sessions WHERE id = 'pull-1'`,
	).Scan(&userID, &imageName, &registry, &tag, &bytesTotal, &layerCount, &requestCount); err != nil {
		t.Fatalf("read migrated pull: %v", err)
	}
	if userID != 7 || imageName != "owner/image" || registry != "ghcr.io" || tag != "v1" ||
		bytesTotal != 12345 || layerCount != 2 || requestCount != 4 {
		t.Fatalf("migrated pull changed: user=%d image=%q registry=%q tag=%q bytes=%d layers=%d requests=%d",
			userID, imageName, registry, tag, bytesTotal, layerCount, requestCount)
	}
}
