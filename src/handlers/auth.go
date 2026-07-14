package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"hubproxy/db"
)

const (
	cookieName   = "hubproxy_session"
	ctxUserKey   = "auth_user"
	ctxSessionID = "auth_session_id"
	loginWindow  = 15 * time.Minute
	maxFailures  = 5 // stricter lockout
)

func extractBearer(c *gin.Context) string {
	h := c.GetHeader("Authorization")
	if strings.HasPrefix(strings.ToLower(h), "bearer ") {
		return strings.TrimSpace(h[7:])
	}
	if t, err := c.Cookie(cookieName); err == nil {
		return t
	}
	return ""
}

func setSessionCookie(c *gin.Context, token string) {
	secure := c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https")
	// Strict + HttpOnly + Secure(when HTTPS) + path-limited
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie(cookieName, token, int(db.SessionTTL.Seconds()), "/api/admin", "", secure, true)
}

func clearSessionCookie(c *gin.Context) {
	secure := c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https")
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie(cookieName, "", -1, "/api/admin", "", secure, true)
}

// SecurityHeadersMiddleware adds baseline browser security headers for console pages/API.
func SecurityHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "no-referrer")
		c.Header("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		// Avoid indexing admin/console/token paths
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/api/admin") || strings.HasPrefix(path, "/admin") {
			c.Header("Cache-Control", "no-store, no-cache, must-revalidate, private")
			c.Header("Pragma", "no-cache")
			c.Header("X-Robots-Tag", "noindex, nofollow, noarchive")
		}
		c.Next()
	}
}

func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := extractBearer(c)
		sess, user, err := db.GetSessionByToken(token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "未登录或会话已过期", "code": "UNAUTHORIZED"})
			return
		}
		c.Set(ctxUserKey, user)
		c.Set(ctxSessionID, sess.ID)
		c.Next()
	}
}

func AdminRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		u, ok := c.Get(ctxUserKey)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "未登录", "code": "UNAUTHORIZED"})
			return
		}
		user := u.(*db.User)
		if user.Role != db.RoleAdmin {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "需要管理员权限", "code": "FORBIDDEN"})
			return
		}
		c.Next()
	}
}

func currentUser(c *gin.Context) *db.User {
	u, _ := c.Get(ctxUserKey)
	if u == nil {
		return nil
	}
	return u.(*db.User)
}

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type registerRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password" binding:"required"`
}

type updateProfileRequest struct {
	Username        string `json:"username"`
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

func AuthLogin(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数无效"})
		return
	}
	ip := c.ClientIP()
	fails, _ := db.CountRecentLoginFailures(ip, loginWindow)
	if fails >= maxFailures {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "登录尝试过多，请稍后再试", "code": "LOGIN_THROTTLED"})
		return
	}

	username := strings.TrimSpace(req.Username)
	if len(username) > 64 || len(req.Password) > 128 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数无效"})
		return
	}

	user, hash, err := db.GetUserByUsername(username)
	if err != nil || !db.CheckPassword(hash, req.Password) {
		_ = db.RecordLoginAttempt(ip, username, false)
		// constant-ish delay against timing / brute force
		time.Sleep(300 * time.Millisecond)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}

	token, _, err := db.CreateSession(user.ID, ip, c.Request.UserAgent())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建会话失败"})
		return
	}
	_ = db.TouchLastLogin(user.ID)
	_ = db.RecordLoginAttempt(ip, req.Username, true)
	_, _ = db.EnsureUserAccessToken(user.ID)
	setSessionCookie(c, token)

	c.JSON(http.StatusOK, gin.H{
		"token": token,
		"user":  user,
	})
}

func AuthLogout(c *gin.Context) {
	token := extractBearer(c)
	if token != "" {
		_ = db.DeleteSessionByToken(token)
	}
	clearSessionCookie(c)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func AuthMe(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"user": currentUser(c)})
}

func AuthRegister(c *gin.Context) {
	admin := db.GlobalRuntime.GetAdmin()
	if !admin.RegisterEnabled {
		c.JSON(http.StatusForbidden, gin.H{"error": "注册已关闭", "code": "REGISTER_DISABLED"})
		return
	}
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数无效"})
		return
	}
	if len(req.Password) < 8 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "密码至少 8 位"})
		return
	}
	user, err := db.CreateUser(req.Username, req.Password, db.RoleUser)
	if err == db.ErrUserExists {
		c.JSON(http.StatusConflict, gin.H{"error": "用户名已存在"})
		return
	}
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"user": user})
}

func AuthChangePassword(c *gin.Context) {
	var req changePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数无效"})
		return
	}
	user := currentUser(c)
	_, hash, err := db.GetUserByUsername(user.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "用户不存在"})
		return
	}
	if !user.MustChangePassword {
		if req.CurrentPassword == "" || !db.CheckPassword(hash, req.CurrentPassword) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "当前密码错误"})
			return
		}
	}
	if len(req.NewPassword) < 8 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "新密码至少 8 位"})
		return
	}
	if err := db.UpdatePassword(user.ID, req.NewPassword); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败"})
		return
	}
	// revoke other sessions
	_ = db.DeleteUserSessions(user.ID)
	token, _, err := db.CreateSession(user.ID, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": true, "message": "密码已更新，请重新登录"})
		return
	}
	setSessionCookie(c, token)
	updated, _ := db.GetUserByID(user.ID)
	c.JSON(http.StatusOK, gin.H{"ok": true, "token": token, "user": updated})
}

func AuthUpdateProfile(c *gin.Context) {
	var req updateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数无效"})
		return
	}
	user := currentUser(c)
	_, hash, err := db.GetUserByUsername(user.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "用户不存在"})
		return
	}

	newUsername := strings.TrimSpace(req.Username)
	changingPassword := strings.TrimSpace(req.NewPassword) != ""
	changingUsername := newUsername != "" && !strings.EqualFold(newUsername, user.Username)

	if user.MustChangePassword && !changingPassword {
		c.JSON(http.StatusBadRequest, gin.H{"error": "首次登录必须修改密码"})
		return
	}
	if !changingPassword && !changingUsername {
		c.JSON(http.StatusBadRequest, gin.H{"error": "未检测到需要修改的内容（用户名或密码）"})
		return
	}

	// 首次强制改密：可不校验当前密码；之后改用户名/密码均需当前密码
	if !user.MustChangePassword {
		if req.CurrentPassword == "" || !db.CheckPassword(hash, req.CurrentPassword) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "当前密码错误"})
			return
		}
	}

	if changingUsername {
		if err := db.UpdateUsername(user.ID, newUsername); err != nil {
			if err == db.ErrUserExists {
				c.JSON(http.StatusConflict, gin.H{"error": "用户名已存在"})
				return
			}
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}

	if changingPassword {
		if len(req.NewPassword) < 8 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "新密码至少 8 位"})
			return
		}
		if err := db.UpdatePassword(user.ID, req.NewPassword); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "更新密码失败"})
			return
		}
	}

	// 改密后吊销旧会话并签发新 token；仅改名也刷新用户信息
	if changingPassword {
		_ = db.DeleteUserSessions(user.ID)
	}
	token, _, err := db.CreateSession(user.ID, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": true, "message": "资料已更新，请重新登录"})
		return
	}
	if changingPassword {
		// already deleted all sessions including none; new session created
	}
	setSessionCookie(c, token)
	updated, _ := db.GetUserByID(user.ID)
	c.JSON(http.StatusOK, gin.H{"ok": true, "token": token, "user": updated})
}

func AuthPublicConfig(c *gin.Context) {
	admin := db.GlobalRuntime.GetAdmin()
	c.JSON(http.StatusOK, gin.H{
		"register_enabled": admin.RegisterEnabled,
	})
}
