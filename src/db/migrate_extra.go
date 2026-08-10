package db

import (
	"fmt"
	"strings"
)

func migrate() error {
	// Base schema: CREATE TABLE IF NOT EXISTS is safe for upgrades.
	// New columns are added in ensureColumns(); indexes that need them come after.
	schema := `
CREATE TABLE IF NOT EXISTS users (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	username TEXT NOT NULL UNIQUE COLLATE NOCASE,
	password_hash TEXT NOT NULL,
	role TEXT NOT NULL DEFAULT 'user' CHECK(role IN ('admin','user')),
	must_change_password INTEGER NOT NULL DEFAULT 0,
	daily_pull_limit INTEGER NOT NULL DEFAULT 30,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	last_login_at TEXT
);

CREATE TABLE IF NOT EXISTS sessions (
	id TEXT PRIMARY KEY,
	user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	token_hash TEXT NOT NULL UNIQUE,
	expires_at TEXT NOT NULL,
	created_at TEXT NOT NULL,
	ip TEXT,
	user_agent TEXT
);
CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_expires ON sessions(expires_at);

CREATE TABLE IF NOT EXISTS settings (
	key TEXT PRIMARY KEY,
	value TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS pull_sessions (
	id TEXT PRIMARY KEY,
	client_ip TEXT NOT NULL,
	image_name TEXT NOT NULL,
	registry TEXT NOT NULL DEFAULT 'docker.io',
	tag TEXT NOT NULL DEFAULT 'latest',
	category TEXT NOT NULL DEFAULT 'library',
	started_at TEXT NOT NULL,
	last_seen_at TEXT NOT NULL,
	completed_at TEXT,
	status TEXT NOT NULL DEFAULT 'active' CHECK(status IN ('active','completed','failed','expired')),
	bytes_total INTEGER NOT NULL DEFAULT 0,
	layer_count INTEGER NOT NULL DEFAULT 0,
	request_count INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_pull_sessions_ip ON pull_sessions(client_ip);
CREATE INDEX IF NOT EXISTS idx_pull_sessions_image ON pull_sessions(image_name);
CREATE INDEX IF NOT EXISTS idx_pull_sessions_category ON pull_sessions(category);
CREATE INDEX IF NOT EXISTS idx_pull_sessions_started ON pull_sessions(started_at);
CREATE INDEX IF NOT EXISTS idx_pull_sessions_registry ON pull_sessions(registry);

CREATE TABLE IF NOT EXISTS pull_events (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	pull_session_id TEXT NOT NULL REFERENCES pull_sessions(id) ON DELETE CASCADE,
	event_type TEXT NOT NULL,
	reference TEXT,
	bytes INTEGER NOT NULL DEFAULT 0,
	status_code INTEGER NOT NULL DEFAULT 0,
	created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_pull_events_session ON pull_events(pull_session_id);
CREATE INDEX IF NOT EXISTS idx_pull_events_created ON pull_events(created_at);

CREATE TABLE IF NOT EXISTS daily_stats (
	day TEXT PRIMARY KEY,
	pull_count INTEGER NOT NULL DEFAULT 0,
	bytes_total INTEGER NOT NULL DEFAULT 0,
	unique_ips INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS login_attempts (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	ip TEXT NOT NULL,
	username TEXT,
	success INTEGER NOT NULL DEFAULT 0,
	created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_login_attempts_ip ON login_attempts(ip, created_at);

CREATE TABLE IF NOT EXISTS user_access_tokens (
	token TEXT PRIMARY KEY,
	user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	status TEXT NOT NULL DEFAULT 'active' CHECK(status IN ('active','revoked')),
	created_at TEXT NOT NULL,
	revoked_at TEXT
);
CREATE INDEX IF NOT EXISTS idx_user_tokens_user ON user_access_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_user_tokens_status ON user_access_tokens(status);

CREATE TABLE IF NOT EXISTS user_ip_whitelist (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	cidr TEXT NOT NULL,
	created_at TEXT NOT NULL,
	UNIQUE(user_id, cidr)
);
CREATE INDEX IF NOT EXISTS idx_user_ip_user ON user_ip_whitelist(user_id);
`
	if _, err := DB.Exec(schema); err != nil {
		return fmt.Errorf("migrate schema: %w", err)
	}
	if err := ensureColumns(); err != nil {
		return fmt.Errorf("migrate columns: %w", err)
	}
	// indexes that depend on new columns
	if _, err := DB.Exec(`
CREATE INDEX IF NOT EXISTS idx_pull_sessions_user ON pull_sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_pull_sessions_token ON pull_sessions(access_token);
`); err != nil {
		return fmt.Errorf("migrate indexes: %w", err)
	}
	return nil
}

func ensureColumns() error {
	cols, err := tableColumns("pull_sessions")
	if err != nil {
		return err
	}
	if !cols["user_id"] {
		if _, err := DB.Exec(`ALTER TABLE pull_sessions ADD COLUMN user_id INTEGER`); err != nil {
			return err
		}
	}
	if !cols["access_token"] {
		if _, err := DB.Exec(`ALTER TABLE pull_sessions ADD COLUMN access_token TEXT`); err != nil {
			return err
		}
	}

	ucols, err := tableColumns("users")
	if err != nil {
		return err
	}
	if !ucols["daily_pull_limit"] {
		if _, err := DB.Exec(`ALTER TABLE users ADD COLUMN daily_pull_limit INTEGER NOT NULL DEFAULT 30`); err != nil {
			return err
		}
	}
	return nil
}

func tableColumns(table string) (map[string]bool, error) {
	rows, err := DB.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt any
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return nil, err
		}
		out[strings.ToLower(name)] = true
	}
	return out, rows.Err()
}
