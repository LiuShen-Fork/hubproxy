package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
	"hubproxy/db"
)

type oauthState struct {
	Mode      string // login | bind
	UserID    int64
	CreatedAt time.Time
}

var (
	oauthStates   = map[string]oauthState{}
	oauthStatesMu sync.Mutex
)

func putOAuthState(mode string, userID int64) (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	key := hex.EncodeToString(b)
	oauthStatesMu.Lock()
	now := time.Now()
	for k, v := range oauthStates {
		if now.Sub(v.CreatedAt) > 15*time.Minute {
			delete(oauthStates, k)
		}
	}
	oauthStates[key] = oauthState{Mode: mode, UserID: userID, CreatedAt: now}
	oauthStatesMu.Unlock()
	return key, nil
}

func takeOAuthState(key string) (oauthState, bool) {
	oauthStatesMu.Lock()
	defer oauthStatesMu.Unlock()
	st, ok := oauthStates[key]
	if ok {
		delete(oauthStates, key)
	}
	if ok && time.Since(st.CreatedAt) > 15*time.Minute {
		return oauthState{}, false
	}
	return st, ok
}

func oauthRedirectURL(c *gin.Context) string {
	scheme := "http"
	if c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	host := c.Request.Host
	if xf := c.GetHeader("X-Forwarded-Host"); xf != "" {
		host = xf
	}
	return scheme + "://" + host + "/api/admin/oauth/callback"
}

func buildOAuthConfig(c *gin.Context, cfg db.OAuthSettings) (*oauth2.Config, error) {
	if !cfg.Enabled || cfg.ClientID == "" || cfg.ClientSecret == "" {
		return nil, fmt.Errorf("OAuth 未配置或未启用")
	}
	if cfg.AuthURL == "" || cfg.TokenURL == "" || cfg.UserInfoURL == "" {
		return nil, fmt.Errorf("请配置 Auth URL / Token URL / UserInfo URL")
	}
	scopes := strings.Fields(cfg.Scopes)
	if len(scopes) == 0 {
		scopes = []string{"openid", "profile", "email"}
	}
	return &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  oauthRedirectURL(c),
		Scopes:       scopes,
		Endpoint: oauth2.Endpoint{
			AuthURL:  cfg.AuthURL,
			TokenURL: cfg.TokenURL,
		},
	}, nil
}

type oauthUserInfo struct {
	Subject   string
	Email     string
	Name      string
	Username  string
	AvatarURL string
}

func fetchOAuthUser(ctx context.Context, cfg db.OAuthSettings, token *oauth2.Token) (*oauthUserInfo, error) {
	client := oauth2.NewClient(ctx, oauth2.StaticTokenSource(token))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.UserInfoURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("userinfo: %s", string(body))
	}
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	info := &oauthUserInfo{}
	for _, k := range []string{"sub", "id", "user_id"} {
		if v, ok := raw[k]; ok {
			info.Subject = fmt.Sprint(v)
			break
		}
	}
	if v, ok := raw["email"].(string); ok {
		info.Email = v
	}
	if v, ok := raw["name"].(string); ok {
		info.Name = v
	}
	for _, k := range []string{"preferred_username", "login", "username"} {
		if v, ok := raw[k].(string); ok && v != "" {
			info.Username = v
			break
		}
	}
	if v, ok := raw["picture"].(string); ok {
		info.AvatarURL = v
	} else if v, ok := raw["avatar_url"].(string); ok {
		info.AvatarURL = v
	}
	if info.Name == "" {
		info.Name = info.Username
	}
	// GitHub-style: try emails if empty
	if info.Email == "" && strings.Contains(cfg.UserInfoURL, "api.github.com") {
		er, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user/emails", nil)
		er.Header.Set("Authorization", "Bearer "+token.AccessToken)
		er.Header.Set("Accept", "application/vnd.github+json")
		if eresp, err := client.Do(er); err == nil {
			defer eresp.Body.Close()
			var emails []map[string]any
			_ = json.NewDecoder(eresp.Body).Decode(&emails)
			for _, e := range emails {
				if prim, _ := e["primary"].(bool); prim {
					if em, ok := e["email"].(string); ok {
						info.Email = em
						break
					}
				}
			}
		}
	}
	if info.Subject == "" {
		return nil, fmt.Errorf("userinfo 缺少 sub/id")
	}
	return info, nil
}

func OAuthStart(c *gin.Context) {
	cfg := db.GlobalRuntime.GetOAuth()
	admin := db.GlobalRuntime.GetAdmin()
	mode := c.DefaultQuery("mode", "login")

	if mode == "bind" {
		if !cfg.Enabled {
			c.JSON(http.StatusForbidden, gin.H{"error": "OAuth 未启用", "code": "OAUTH_DISABLED"})
			return
		}
		token := extractBearer(c)
		_, user, err := db.GetSessionByToken(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "请先登录后再绑定", "code": "UNAUTHORIZED"})
			return
		}
		oc, err := buildOAuthConfig(c, cfg)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "code": "OAUTH_NOT_READY"})
			return
		}
		state, err := putOAuthState("bind", user.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "state 生成失败"})
			return
		}
		c.Redirect(http.StatusFound, oc.AuthCodeURL(state, oauth2.AccessTypeOnline))
		return
	}

	if !cfg.Enabled || !admin.OAuthLoginEnabled {
		c.JSON(http.StatusForbidden, gin.H{"error": "OAuth 登录未开启", "code": "OAUTH_LOGIN_DISABLED"})
		return
	}
	oc, err := buildOAuthConfig(c, cfg)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "code": "OAUTH_NOT_READY"})
		return
	}
	state, err := putOAuthState("login", 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "state 生成失败"})
		return
	}
	c.Redirect(http.StatusFound, oc.AuthCodeURL(state, oauth2.AccessTypeOnline))
}

func OAuthCallback(c *gin.Context) {
	cfg := db.GlobalRuntime.GetOAuth()
	admin := db.GlobalRuntime.GetAdmin()
	code := c.Query("code")
	stateKey := c.Query("state")
	if code == "" || stateKey == "" {
		redirectOAuthError(c, "缺少 code 或 state")
		return
	}
	st, ok := takeOAuthState(stateKey)
	if !ok {
		redirectOAuthError(c, "state 无效或已过期")
		return
	}
	oc, err := buildOAuthConfig(c, cfg)
	if err != nil {
		redirectOAuthError(c, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
	defer cancel()
	tok, err := oc.Exchange(ctx, code)
	if err != nil {
		redirectOAuthError(c, "换取 token 失败: "+err.Error())
		return
	}
	info, err := fetchOAuthUser(ctx, cfg, tok)
	if err != nil {
		redirectOAuthError(c, "获取用户信息失败: "+err.Error())
		return
	}
	provider := "oauth2"

	if st.Mode == "bind" {
		if err := db.UpsertOAuthBinding(st.UserID, provider, info.Subject, info.Email, info.Name, info.AvatarURL); err != nil {
			redirectOAuthError(c, err.Error())
			return
		}
		c.Redirect(http.StatusFound, "/admin/change-password?oauth=bound")
		return
	}

	binding, err := db.GetOAuthBinding(provider, info.Subject)
	if err != nil {
		redirectOAuthError(c, err.Error())
		return
	}
	var user *db.User
	if binding != nil {
		user, err = db.GetUserByID(binding.UserID)
		if err != nil {
			redirectOAuthError(c, "绑定用户不存在")
			return
		}
		_ = db.UpsertOAuthBinding(user.ID, provider, info.Subject, info.Email, info.Name, info.AvatarURL)
	} else {
		if !admin.OAuthRegisterEnabled {
			redirectOAuthError(c, "该第三方账号未绑定，且管理员未开启 OAuth 注册")
			return
		}
		preferred := info.Username
		if preferred == "" {
			preferred = info.Name
		}
		user, err = db.CreateOAuthUser(preferred, info.Email)
		if err != nil {
			redirectOAuthError(c, "创建用户失败: "+err.Error())
			return
		}
		if err := db.UpsertOAuthBinding(user.ID, provider, info.Subject, info.Email, info.Name, info.AvatarURL); err != nil {
			redirectOAuthError(c, err.Error())
			return
		}
	}

	sessionToken, _, err := db.CreateSession(user.ID, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		redirectOAuthError(c, "创建会话失败")
		return
	}
	_ = db.TouchLastLogin(user.ID)
	setSessionCookie(c, sessionToken)

	dest := "/admin"
	if user.Role != db.RoleAdmin {
		dest = "/admin/user"
	}
	if user.MustChangePassword {
		dest = "/admin/change-password"
	}
	c.Redirect(http.StatusFound, dest+"#oauth_token="+url.QueryEscape(sessionToken))
}

func redirectOAuthError(c *gin.Context, msg string) {
	c.Redirect(http.StatusFound, "/admin/login?oauth_error="+url.QueryEscape(msg))
}

func OAuthListBindings(c *gin.Context) {
	u := currentUser(c)
	list, err := db.ListOAuthBindings(u.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": list})
}

func OAuthUnbind(c *gin.Context) {
	u := currentUser(c)
	provider := strings.TrimSpace(c.Query("provider"))
	if provider == "" {
		var req struct {
			Provider string `json:"provider"`
		}
		_ = c.ShouldBindJSON(&req)
		provider = strings.TrimSpace(req.Provider)
	}
	if provider == "" {
		provider = "oauth2"
	}
	if err := db.DeleteOAuthBinding(u.ID, provider); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	list, _ := db.ListOAuthBindings(u.ID)
	c.JSON(http.StatusOK, gin.H{"items": list})
}
