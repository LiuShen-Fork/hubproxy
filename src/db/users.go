package db

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	RoleAdmin = "admin"
	RoleUser  = "user"

	DefaultAdminUsername = "admin"
	DefaultAdminPassword = "admin12346"
	SessionTTL           = 12 * time.Hour
	BcryptCost           = 12
	DefaultDailyPullLimit = 30
)

var (
	ErrUserNotFound      = errors.New("user not found")
	ErrInvalidCredential = errors.New("invalid username or password")
	ErrUserExists        = errors.New("username already exists")
	ErrSessionInvalid    = errors.New("session invalid or expired")
)

type User struct {
	ID                 int64  `json:"id"`
	Username           string `json:"username"`
	Role               string `json:"role"`
	MustChangePassword bool   `json:"must_change_password"`
	DailyPullLimit     int    `json:"daily_pull_limit"`
	CreatedAt          string `json:"created_at"`
	UpdatedAt          string `json:"updated_at"`
	LastLoginAt        string `json:"last_login_at,omitempty"`
}

type Session struct {
	ID        string `json:"id"`
	UserID    int64  `json:"user_id"`
	TokenHash string `json:"-"`
	ExpiresAt string `json:"expires_at"`
	CreatedAt string `json:"created_at"`
	IP        string `json:"ip,omitempty"`
	UserAgent string `json:"user_agent,omitempty"`
}

func EnsureDefaultAdmin() error {
	var count int
	if err := DB.QueryRow(`SELECT COUNT(*) FROM users WHERE role = ?`, RoleAdmin).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	hash, err := HashPassword(DefaultAdminPassword)
	if err != nil {
		return err
	}
	now := Now()
	res, err := DB.Exec(
		`INSERT INTO users (username, password_hash, role, must_change_password, daily_pull_limit, created_at, updated_at)
		 VALUES (?, ?, ?, 1, ?, ?, ?)`,
		DefaultAdminUsername, hash, RoleAdmin, DefaultDailyPullLimit, now, now,
	)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	_, _ = EnsureUserAccessToken(id)
	return nil
}

func HashPassword(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), BcryptCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func CreateUser(username, password, role string) (*User, error) {
	username = strings.TrimSpace(username)
	if username == "" || len(password) < 8 {
		return nil, fmt.Errorf("invalid username or password too short")
	}
	if role != RoleAdmin && role != RoleUser {
		role = RoleUser
	}
	hash, err := HashPassword(password)
	if err != nil {
		return nil, err
	}
	now := Now()
	res, err := DB.Exec(
		`INSERT INTO users (username, password_hash, role, must_change_password, daily_pull_limit, created_at, updated_at)
		 VALUES (?, ?, ?, 0, ?, ?, ?)`,
		username, hash, role, DefaultDailyPullLimit, now, now,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return nil, ErrUserExists
		}
		return nil, err
	}
	id, _ := res.LastInsertId()
	_, _ = EnsureUserAccessToken(id)
	return GetUserByID(id)
}

func scanUser(row interface {
	Scan(dest ...any) error
}, withHash bool) (*User, string, error) {
	u := &User{}
	var lastLogin sql.NullString
	var mustChange int
	var hash string
	var err error
	if withHash {
		err = row.Scan(&u.ID, &u.Username, &hash, &u.Role, &mustChange, &u.DailyPullLimit, &u.CreatedAt, &u.UpdatedAt, &lastLogin)
	} else {
		err = row.Scan(&u.ID, &u.Username, &u.Role, &mustChange, &u.DailyPullLimit, &u.CreatedAt, &u.UpdatedAt, &lastLogin)
	}
	if err != nil {
		return nil, "", err
	}
	u.MustChangePassword = mustChange == 1
	if u.DailyPullLimit <= 0 {
		u.DailyPullLimit = DefaultDailyPullLimit
	}
	if lastLogin.Valid {
		u.LastLoginAt = lastLogin.String
	}
	return u, hash, nil
}

func GetUserByID(id int64) (*User, error) {
	row := DB.QueryRow(
		`SELECT id, username, role, must_change_password, COALESCE(daily_pull_limit, 30), created_at, updated_at, last_login_at
		 FROM users WHERE id = ?`, id,
	)
	u, _, err := scanUser(row, false)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return u, nil
}

func GetUserByUsername(username string) (*User, string, error) {
	row := DB.QueryRow(
		`SELECT id, username, password_hash, role, must_change_password, COALESCE(daily_pull_limit, 30), created_at, updated_at, last_login_at
		 FROM users WHERE username = ? COLLATE NOCASE`, username,
	)
	u, hash, err := scanUser(row, true)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, "", ErrUserNotFound
	}
	if err != nil {
		return nil, "", err
	}
	return u, hash, nil
}

func ListUsers() ([]User, error) {
	rows, err := DB.Query(
		`SELECT id, username, role, must_change_password, COALESCE(daily_pull_limit, 30), created_at, updated_at, last_login_at
		 FROM users ORDER BY id ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []User
	for rows.Next() {
		u, _, err := scanUser(rows, false)
		if err != nil {
			return nil, err
		}
		list = append(list, *u)
	}
	return list, rows.Err()
}

func UpdateDailyPullLimit(userID int64, limit int) error {
	if limit < 0 {
		return fmt.Errorf("每日拉取上限不能为负数")
	}
	if limit > 100000 {
		return fmt.Errorf("每日拉取上限过大")
	}
	_, err := DB.Exec(`UPDATE users SET daily_pull_limit = ?, updated_at = ? WHERE id = ?`, limit, Now(), userID)
	return err
}

func UpdatePassword(userID int64, newPassword string) error {
	if len(newPassword) < 8 {
		return fmt.Errorf("password too short")
	}
	hash, err := HashPassword(newPassword)
	if err != nil {
		return err
	}
	_, err = DB.Exec(
		`UPDATE users SET password_hash = ?, must_change_password = 0, updated_at = ? WHERE id = ?`,
		hash, Now(), userID,
	)
	return err
}

func UpdateUsername(userID int64, username string) error {
	username = strings.TrimSpace(username)
	if username == "" || len(username) < 2 || len(username) > 32 {
		return fmt.Errorf("用户名长度需为 2-32 个字符")
	}
	for _, r := range username {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			continue
		}
		return fmt.Errorf("用户名仅允许字母、数字、下划线和连字符")
	}
	var existingID int64
	err := DB.QueryRow(`SELECT id FROM users WHERE username = ? COLLATE NOCASE AND id != ?`, username, userID).Scan(&existingID)
	if err == nil {
		return ErrUserExists
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	_, err = DB.Exec(`UPDATE users SET username = ?, updated_at = ? WHERE id = ?`, username, Now(), userID)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return ErrUserExists
		}
		return err
	}
	return nil
}

func UpdateUserRole(userID int64, role string) error {
	if role != RoleAdmin && role != RoleUser {
		return fmt.Errorf("invalid role")
	}
	_, err := DB.Exec(`UPDATE users SET role = ?, updated_at = ? WHERE id = ?`, role, Now(), userID)
	return err
}

func DeleteUser(userID int64) error {
	var role string
	if err := DB.QueryRow(`SELECT role FROM users WHERE id = ?`, userID).Scan(&role); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrUserNotFound
		}
		return err
	}
	if role == RoleAdmin {
		var adminCount int
		if err := DB.QueryRow(`SELECT COUNT(*) FROM users WHERE role = ?`, RoleAdmin).Scan(&adminCount); err != nil {
			return err
		}
		if adminCount <= 1 {
			return fmt.Errorf("cannot delete the last admin")
		}
	}
	_, err := DB.Exec(`DELETE FROM users WHERE id = ?`, userID)
	return err
}

func TouchLastLogin(userID int64) error {
	_, err := DB.Exec(`UPDATE users SET last_login_at = ?, updated_at = ? WHERE id = ?`, Now(), Now(), userID)
	return err
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func GenerateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func CreateSession(userID int64, ip, userAgent string) (rawToken string, session *Session, err error) {
	rawToken, err = GenerateToken()
	if err != nil {
		return "", nil, err
	}
	id, err := GenerateToken()
	if err != nil {
		return "", nil, err
	}
	now := time.Now().UTC()
	expires := now.Add(SessionTTL)
	s := &Session{
		ID:        id,
		UserID:    userID,
		TokenHash: hashToken(rawToken),
		ExpiresAt: expires.Format(time.RFC3339Nano),
		CreatedAt: now.Format(time.RFC3339Nano),
		IP:        ip,
		UserAgent: truncate(userAgent, 512),
	}
	_, err = DB.Exec(
		`INSERT INTO sessions (id, user_id, token_hash, expires_at, created_at, ip, user_agent)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		s.ID, s.UserID, s.TokenHash, s.ExpiresAt, s.CreatedAt, s.IP, s.UserAgent,
	)
	if err != nil {
		return "", nil, err
	}
	return rawToken, s, nil
}

func GetSessionByToken(rawToken string) (*Session, *User, error) {
	if rawToken == "" {
		return nil, nil, ErrSessionInvalid
	}
	tokenHash := hashToken(rawToken)
	s := &Session{}
	err := DB.QueryRow(
		`SELECT id, user_id, token_hash, expires_at, created_at, ip, user_agent
		 FROM sessions WHERE token_hash = ?`, tokenHash,
	).Scan(&s.ID, &s.UserID, &s.TokenHash, &s.ExpiresAt, &s.CreatedAt, &s.IP, &s.UserAgent)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, ErrSessionInvalid
	}
	if err != nil {
		return nil, nil, err
	}
	exp, err := ParseTime(s.ExpiresAt)
	if err != nil || time.Now().UTC().After(exp) {
		_ = DeleteSession(s.ID)
		return nil, nil, ErrSessionInvalid
	}
	u, err := GetUserByID(s.UserID)
	if err != nil {
		return nil, nil, ErrSessionInvalid
	}
	return s, u, nil
}

func DeleteSession(id string) error {
	_, err := DB.Exec(`DELETE FROM sessions WHERE id = ?`, id)
	return err
}

func DeleteSessionByToken(rawToken string) error {
	_, err := DB.Exec(`DELETE FROM sessions WHERE token_hash = ?`, hashToken(rawToken))
	return err
}

func DeleteUserSessions(userID int64) error {
	_, err := DB.Exec(`DELETE FROM sessions WHERE user_id = ?`, userID)
	return err
}

func CleanupExpiredSessions() error {
	_, err := DB.Exec(`DELETE FROM sessions WHERE expires_at < ?`, Now())
	return err
}

func CountRecentLoginFailures(ip string, window time.Duration) (int, error) {
	since := time.Now().UTC().Add(-window).Format(time.RFC3339Nano)
	var n int
	err := DB.QueryRow(
		`SELECT COUNT(*) FROM login_attempts WHERE ip = ? AND success = 0 AND created_at >= ?`,
		ip, since,
	).Scan(&n)
	return n, err
}

func RecordLoginAttempt(ip, username string, success bool) error {
	ok := 0
	if success {
		ok = 1
	}
	_, err := DB.Exec(
		`INSERT INTO login_attempts (ip, username, success, created_at) VALUES (?, ?, ?, ?)`,
		ip, username, ok, Now(),
	)
	return err
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
