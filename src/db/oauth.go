package db

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

type OAuthBinding struct {
	ID          int64  `json:"id"`
	UserID      int64  `json:"user_id"`
	Provider    string `json:"provider"`
	Subject     string `json:"subject"`
	Email       string `json:"email,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
	AvatarURL   string `json:"avatar_url,omitempty"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

func GetOAuthBinding(provider, subject string) (*OAuthBinding, error) {
	var b OAuthBinding
	err := DB.QueryRow(
		`SELECT id, user_id, provider, subject, COALESCE(email,''), COALESCE(display_name,''),
		        COALESCE(avatar_url,''), created_at, updated_at
		 FROM oauth_bindings WHERE provider = ? AND subject = ?`,
		provider, subject,
	).Scan(&b.ID, &b.UserID, &b.Provider, &b.Subject, &b.Email, &b.DisplayName, &b.AvatarURL, &b.CreatedAt, &b.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &b, nil
}

func ListOAuthBindings(userID int64) ([]OAuthBinding, error) {
	rows, err := DB.Query(
		`SELECT id, user_id, provider, subject, COALESCE(email,''), COALESCE(display_name,''),
		        COALESCE(avatar_url,''), created_at, updated_at
		 FROM oauth_bindings WHERE user_id = ? ORDER BY id ASC`, userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []OAuthBinding
	for rows.Next() {
		var b OAuthBinding
		if err := rows.Scan(&b.ID, &b.UserID, &b.Provider, &b.Subject, &b.Email, &b.DisplayName, &b.AvatarURL, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, b)
	}
	if list == nil {
		list = []OAuthBinding{}
	}
	return list, rows.Err()
}

func UpsertOAuthBinding(userID int64, provider, subject, email, displayName, avatar string) error {
	provider = strings.TrimSpace(provider)
	subject = strings.TrimSpace(subject)
	if provider == "" || subject == "" {
		return fmt.Errorf("invalid oauth identity")
	}
	now := Now()
	existing, err := GetOAuthBinding(provider, subject)
	if err != nil {
		return err
	}
	if existing != nil {
		if existing.UserID != userID {
			return fmt.Errorf("该第三方账号已绑定其他用户")
		}
		_, err = DB.Exec(
			`UPDATE oauth_bindings SET email = ?, display_name = ?, avatar_url = ?, updated_at = ? WHERE id = ?`,
			email, displayName, avatar, now, existing.ID,
		)
		return err
	}
	_, err = DB.Exec(
		`INSERT INTO oauth_bindings (user_id, provider, subject, email, display_name, avatar_url, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		userID, provider, subject, email, displayName, avatar, now, now,
	)
	return err
}

func DeleteOAuthBinding(userID int64, provider string) error {
	_, err := DB.Exec(`DELETE FROM oauth_bindings WHERE user_id = ? AND provider = ?`, userID, provider)
	return err
}

// CreateOAuthUser creates a local user for OAuth registration.
func CreateOAuthUser(preferredUsername, email string) (*User, error) {
	base := preferredUsername
	if base == "" {
		if email != "" {
			base = strings.Split(email, "@")[0]
		} else {
			base = "user"
		}
	}
	// sanitize
	var b strings.Builder
	for _, r := range base {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			b.WriteRune(r)
		}
	}
	base = b.String()
	if len(base) < 2 {
		base = "user"
	}
	if len(base) > 24 {
		base = base[:24]
	}

	// unique username
	name := base
	for i := 0; i < 50; i++ {
		if i > 0 {
			name = fmt.Sprintf("%s%d", base, i)
		}
		_, _, err := GetUserByUsername(name)
		if err == ErrUserNotFound {
			break
		}
		if err != nil && !errors.Is(err, ErrUserNotFound) {
			return nil, err
		}
		if i == 49 {
			return nil, fmt.Errorf("无法生成唯一用户名")
		}
	}

	// 随机密码；用户首次登录时必须自行替换
	raw, err := GenerateToken()
	if err != nil {
		return nil, err
	}
	// 取十六进制串的前 24 位作为密码
	pwd := raw
	if len(pwd) > 24 {
		pwd = pwd[:24]
	}
	u, err := CreateUser(name, pwd+"Aa1!", RoleUser)
	if err != nil {
		return nil, err
	}
	// oauth 建号时密码是随机生成的，用户并不知道。必须强制其走一次改密，
	// 否则该账号永远无法用用户名密码登录。
	// 写失败必须整体返回错误：否则账号创建"成功"，但 must_change_password
	// 仍为 0，用户会被静默放进控制台，本次安全修复形同失效。
	if _, err := DB.Exec(`UPDATE users SET must_change_password = 1 WHERE id = ?`, u.ID); err != nil {
		return nil, err
	}
	return GetUserByID(u.ID)
}
