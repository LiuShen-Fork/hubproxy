package db

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"net/smtp"
	"strings"
	"time"
)

// Ensure email_codes table exists (called from migrate).
func ensureEmailCodesTable() error {
	_, err := DB.Exec(`
CREATE TABLE IF NOT EXISTS email_codes (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	email TEXT NOT NULL,
	code TEXT NOT NULL,
	purpose TEXT NOT NULL DEFAULT 'register',
	expires_at TEXT NOT NULL,
	used INTEGER NOT NULL DEFAULT 0,
	created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_email_codes_email ON email_codes(email, purpose);
`)
	return err
}

func CreateEmailCode(email, purpose string, ttl time.Duration) (string, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" || !strings.Contains(email, "@") {
		return "", fmt.Errorf("邮箱无效")
	}
	if purpose == "" {
		purpose = "register"
	}
	b := make([]byte, 3)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	// 6-digit numeric-ish code
	n := int(b[0])<<16 | int(b[1])<<8 | int(b[2])
	code := fmt.Sprintf("%06d", n%1000000)
	now := time.Now().UTC()
	exp := now.Add(ttl).Format(time.RFC3339Nano)
	_, err := DB.Exec(
		`INSERT INTO email_codes (email, code, purpose, expires_at, used, created_at) VALUES (?, ?, ?, ?, 0, ?)`,
		email, code, purpose, exp, now.Format(time.RFC3339Nano),
	)
	if err != nil {
		return "", err
	}
	return code, nil
}

func VerifyEmailCode(email, code, purpose string) error {
	email = strings.TrimSpace(strings.ToLower(email))
	code = strings.TrimSpace(code)
	if purpose == "" {
		purpose = "register"
	}
	var id int64
	var exp string
	var used int
	err := DB.QueryRow(
		`SELECT id, expires_at, used FROM email_codes
		 WHERE email = ? AND code = ? AND purpose = ?
		 ORDER BY id DESC LIMIT 1`,
		email, code, purpose,
	).Scan(&id, &exp, &used)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("验证码错误")
	}
	if err != nil {
		return err
	}
	if used == 1 {
		return fmt.Errorf("验证码已使用")
	}
	t, err := time.Parse(time.RFC3339Nano, exp)
	if err != nil || time.Now().UTC().After(t) {
		return fmt.Errorf("验证码已过期")
	}
	_, _ = DB.Exec(`UPDATE email_codes SET used = 1 WHERE id = ?`, id)
	return nil
}

func SendSMTPMail(cfg EmailSettings, to, subject, body string) error {
	if cfg.SMTPHost == "" || cfg.SMTPPort <= 0 {
		return fmt.Errorf("SMTP 未配置")
	}
	from := cfg.From
	if from == "" {
		from = cfg.Username
	}
	if from == "" {
		return fmt.Errorf("发件人地址未配置")
	}
	addr := fmt.Sprintf("%s:%d", cfg.SMTPHost, cfg.SMTPPort)
	fromHeader := from
	if cfg.FromName != "" {
		fromHeader = fmt.Sprintf("%s <%s>", cfg.FromName, from)
	}
	msg := strings.Join([]string{
		"From: " + fromHeader,
		"To: " + to,
		"Subject: " + subject,
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"",
		body,
	}, "\r\n")

	var auth smtp.Auth
	if cfg.Username != "" {
		auth = smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.SMTPHost)
	}

	// Port 465 typically needs SSL; net/smtp SendMail uses STARTTLS when available on 587.
	// For simplicity use SendMail which supports STARTTLS upgrade.
	if err := smtp.SendMail(addr, auth, from, []string{to}, []byte(msg)); err != nil {
		return fmt.Errorf("发送失败: %w", err)
	}
	return nil
}

func RandomPassword() (string, error) {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b) + "Aa1", nil
}
