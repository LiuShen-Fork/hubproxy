package db

import (
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"math/big"
	"net"
	"strings"
)

const AccessTokenLen = 8

var tokenAlphabet = []byte("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

type UserAccessToken struct {
	Token     string `json:"token"`
	UserID    int64  `json:"user_id"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
	RevokedAt string `json:"revoked_at,omitempty"`
}

func generateAccessToken() (string, error) {
	b := make([]byte, AccessTokenLen)
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(tokenAlphabet))))
		if err != nil {
			return "", err
		}
		b[i] = tokenAlphabet[n.Int64()]
	}
	return string(b), nil
}

func isTokenFormat(s string) bool {
	if len(s) != AccessTokenLen {
		return false
	}
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			continue
		}
		return false
	}
	return true
}

// IsAccessTokenFormat exposes format check for handlers.
func IsAccessTokenFormat(s string) bool {
	return isTokenFormat(s)
}

func tokenExists(token string) (bool, error) {
	var n int
	err := DB.QueryRow(`SELECT COUNT(*) FROM user_access_tokens WHERE token = ?`, token).Scan(&n)
	return n > 0, err
}

func GenerateUniqueAccessToken() (string, error) {
	for i := 0; i < 32; i++ {
		t, err := generateAccessToken()
		if err != nil {
			return "", err
		}
		exists, err := tokenExists(t)
		if err != nil {
			return "", err
		}
		// never reuse any historical token (including revoked)
		if !exists {
			return t, nil
		}
	}
	return "", fmt.Errorf("failed to generate unique access token")
}

func EnsureUserAccessToken(userID int64) (*UserAccessToken, error) {
	var t UserAccessToken
	var revoked sql.NullString
	err := DB.QueryRow(
		`SELECT token, user_id, status, created_at, revoked_at FROM user_access_tokens
		 WHERE user_id = ? AND status = 'active' ORDER BY created_at DESC LIMIT 1`,
		userID,
	).Scan(&t.Token, &t.UserID, &t.Status, &t.CreatedAt, &revoked)
	if err == nil {
		if revoked.Valid {
			t.RevokedAt = revoked.String
		}
		return &t, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	return CreateUserAccessToken(userID)
}

func CreateUserAccessToken(userID int64) (*UserAccessToken, error) {
	token, err := GenerateUniqueAccessToken()
	if err != nil {
		return nil, err
	}
	now := Now()
	_, err = DB.Exec(
		`INSERT INTO user_access_tokens (token, user_id, status, created_at) VALUES (?, ?, 'active', ?)`,
		token, userID, now,
	)
	if err != nil {
		return nil, err
	}
	return &UserAccessToken{Token: token, UserID: userID, Status: "active", CreatedAt: now}, nil
}

func ResetUserAccessToken(userID int64) (*UserAccessToken, error) {
	now := Now()
	_, err := DB.Exec(
		`UPDATE user_access_tokens SET status = 'revoked', revoked_at = ? WHERE user_id = ? AND status = 'active'`,
		now, userID,
	)
	if err != nil {
		return nil, err
	}
	return CreateUserAccessToken(userID)
}

func GetActiveToken(token string) (*UserAccessToken, error) {
	if !isTokenFormat(token) {
		return nil, sql.ErrNoRows
	}
	var t UserAccessToken
	var revoked sql.NullString
	err := DB.QueryRow(
		`SELECT token, user_id, status, created_at, revoked_at FROM user_access_tokens
		 WHERE token = ? AND status = 'active'`, token,
	).Scan(&t.Token, &t.UserID, &t.Status, &t.CreatedAt, &revoked)
	if err != nil {
		return nil, err
	}
	if revoked.Valid {
		t.RevokedAt = revoked.String
	}
	return &t, nil
}

func ListUserIPWhitelist(userID int64) ([]string, error) {
	rows, err := DB.Query(`SELECT cidr FROM user_ip_whitelist WHERE user_id = ? ORDER BY id ASC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, err
		}
		list = append(list, c)
	}
	if list == nil {
		list = []string{}
	}
	return list, rows.Err()
}

func SetUserIPWhitelist(userID int64, cidrs []string) error {
	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM user_ip_whitelist WHERE user_id = ?`, userID); err != nil {
		return err
	}
	now := Now()
	seen := map[string]bool{}
	for _, c := range cidrs {
		c = strings.TrimSpace(c)
		if c == "" || seen[c] {
			continue
		}
		if !validCIDROrIP(c) {
			return fmt.Errorf("无效 IP/CIDR: %s", c)
		}
		seen[c] = true
		if _, err := tx.Exec(
			`INSERT INTO user_ip_whitelist (user_id, cidr, created_at) VALUES (?, ?, ?)`,
			userID, c, now,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func AddUserIPWhitelist(userID int64, cidr string) error {
	cidr = strings.TrimSpace(cidr)
	if !validCIDROrIP(cidr) {
		return fmt.Errorf("无效 IP/CIDR")
	}
	_, err := DB.Exec(
		`INSERT OR IGNORE INTO user_ip_whitelist (user_id, cidr, created_at) VALUES (?, ?, ?)`,
		userID, cidr, Now(),
	)
	return err
}

func RemoveUserIPWhitelist(userID int64, cidr string) error {
	_, err := DB.Exec(`DELETE FROM user_ip_whitelist WHERE user_id = ? AND cidr = ?`, userID, strings.TrimSpace(cidr))
	return err
}

func validCIDROrIP(s string) bool {
	if strings.Contains(s, "/") {
		_, _, err := net.ParseCIDR(s)
		return err == nil
	}
	return net.ParseIP(s) != nil
}

// CheckUserIPAllowed returns true if user has no whitelist (allow all) or IP matches.
func CheckUserIPAllowed(userID int64, ip string) (bool, error) {
	list, err := ListUserIPWhitelist(userID)
	if err != nil {
		return false, err
	}
	if len(list) == 0 {
		return true, nil
	}
	clean := ip
	if host, _, err := net.SplitHostPort(ip); err == nil {
		clean = host
	}
	parsed := net.ParseIP(clean)
	if parsed == nil {
		return false, nil
	}
	for _, item := range list {
		item = strings.TrimSpace(item)
		if !strings.Contains(item, "/") {
			if strings.Contains(item, ":") {
				item += "/128"
			} else {
				item += "/32"
			}
		}
		_, n, err := net.ParseCIDR(item)
		if err != nil {
			continue
		}
		if n.Contains(parsed) {
			return true, nil
		}
	}
	return false, nil
}
