package db

import (
	"database/sql"
	"strings"
	"testing"
)

func useTestDB(t *testing.T) {
	t.Helper()
	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	database.SetMaxOpenConns(1)
	previous := DB
	DB = database
	t.Cleanup(func() {
		_ = database.Close()
		DB = previous
	})
}

func TestGeneratedAccessTokenIsDockerPathSafe(t *testing.T) {
	for i := 0; i < 256; i++ {
		token, err := generateAccessToken()
		if err != nil {
			t.Fatalf("generate token: %v", err)
		}
		if len(token) != AccessTokenLen {
			t.Fatalf("token length = %d, want %d", len(token), AccessTokenLen)
		}
		if token != strings.ToLower(token) {
			t.Fatalf("generated token contains uppercase characters: %q", token)
		}
		for _, r := range token {
			if (r < 'a' || r > 'z') && (r < '0' || r > '9') {
				t.Fatalf("generated token contains invalid character %q", r)
			}
		}
	}
}

func TestAccessTokensAreUniqueAcrossUsersAndHistory(t *testing.T) {
	useTestDB(t)
	if err := migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	now := Now()
	for _, username := range []string{"user1", "user2"} {
		if _, err := DB.Exec(
			`INSERT INTO users (username, password_hash, role, created_at, updated_at) VALUES (?, 'hash', 'user', ?, ?)`,
			username, now, now,
		); err != nil {
			t.Fatalf("insert %s: %v", username, err)
		}
	}

	first, err := CreateUserAccessToken(1)
	if err != nil {
		t.Fatalf("create first token: %v", err)
	}
	second, err := CreateUserAccessToken(2)
	if err != nil {
		t.Fatalf("create second token: %v", err)
	}
	reset, err := ResetUserAccessToken(1)
	if err != nil {
		t.Fatalf("reset first token: %v", err)
	}

	if first.Token == second.Token || first.Token == reset.Token || second.Token == reset.Token {
		t.Fatalf("tokens were reused: %q, %q, %q", first.Token, second.Token, reset.Token)
	}
	var total, distinct int
	if err := DB.QueryRow(`SELECT COUNT(*), COUNT(DISTINCT token) FROM user_access_tokens`).Scan(&total, &distinct); err != nil {
		t.Fatalf("count tokens: %v", err)
	}
	if total != 3 || distinct != total {
		t.Fatalf("token rows = %d, distinct = %d", total, distinct)
	}
	if _, err := DB.Exec(
		`INSERT INTO user_access_tokens (token, user_id, status, created_at) VALUES (?, 2, 'active', ?)`,
		first.Token, now,
	); err == nil {
		t.Fatal("database accepted the same token for a different user")
	}
}

func TestLegacyMixedCaseTokenRemainsValid(t *testing.T) {
	if !IsAccessTokenFormat("8keew0M2") {
		t.Fatal("legacy mixed-case token format must remain accepted")
	}
}
