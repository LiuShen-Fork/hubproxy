package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"hubproxy/db"
)

func RegisterAdminRoutes(r *gin.Engine) {
	// Public
	r.GET("/api/admin/public-config", AuthPublicConfig)
	r.POST("/api/admin/login", AuthLogin)
	r.POST("/api/admin/register", AuthRegister)
	r.POST("/api/admin/register/send-code", AuthSendRegisterCode)
	r.GET("/api/admin/oauth/start", OAuthStart)
	r.GET("/api/admin/oauth/callback", OAuthCallback)

	// Authenticated (user + admin)
	auth := r.Group("/api/admin")
	auth.Use(AuthRequired())
	{
		auth.POST("/logout", AuthLogout)
		auth.GET("/me", AuthMe)
		auth.POST("/change-password", AuthChangePassword)
		auth.POST("/profile", AuthUpdateProfile)

		// User console — explicit full paths (avoid nested group routing issues)
		auth.GET("/user/dashboard", UserDashboard)
		auth.GET("/user/quota", UserQuota)
		auth.GET("/user/pulls", UserListPulls)
		auth.GET("/user/token", UserGetToken)
		auth.POST("/user/token/reset", UserResetToken)
		auth.GET("/user/ip-whitelist", UserGetIPWhitelist)
		auth.PUT("/user/ip-whitelist", UserPutIPWhitelist)
		auth.POST("/user/ip-whitelist", UserAddIPWhitelist)
		auth.DELETE("/user/ip-whitelist", UserRemoveIPWhitelist)
		auth.GET("/user/guide", UserGuide)
		auth.GET("/user/oauth/bindings", OAuthListBindings)
		auth.DELETE("/user/oauth/bindings", OAuthUnbind)
		// bind uses same /oauth/start?mode=bind with auth header/cookie
	}

	// Admin only
	adminOnly := r.Group("/api/admin")
	adminOnly.Use(AuthRequired(), AdminRequired())
	{
		adminOnly.GET("/dashboard", AdminDashboard)
		adminOnly.GET("/pulls", AdminListPulls)
		adminOnly.GET("/pulls/:id", AdminGetPull)
		adminOnly.GET("/images", AdminListImages)
		adminOnly.GET("/ips", AdminListIPs)

		adminOnly.GET("/users", AdminListUsers)
		adminOnly.POST("/users", AdminCreateUser)
		adminOnly.PATCH("/users/:id", AdminUpdateUser)
		adminOnly.DELETE("/users/:id", AdminDeleteUser)

		adminOnly.GET("/settings", AdminGetSettings)
		adminOnly.PUT("/settings/rate-limit", AdminPutRateLimit)
		adminOnly.PUT("/settings/security", AdminPutSecurity)
		adminOnly.PUT("/settings/access", AdminPutAccess)
		adminOnly.PUT("/settings/admin", AdminPutAdmin)
		adminOnly.PUT("/settings/site", AdminPutSite)
		adminOnly.PUT("/settings/oauth", AdminPutOAuth)
		adminOnly.PUT("/settings/email", AdminPutEmail)
		adminOnly.POST("/settings/email/test", AdminTestEmail)
		adminOnly.PUT("/settings/pull-session", AdminPutPullSession)
		adminOnly.PUT("/settings/features", AdminPutFeatures)
		adminOnly.PUT("/settings/registries", AdminPutRegistries)

		adminOnly.POST("/security/blacklist", AdminAddBlackIP)
		adminOnly.DELETE("/security/blacklist", AdminRemoveBlackIP)
		adminOnly.POST("/security/whitelist", AdminAddWhiteIP)
		adminOnly.DELETE("/security/whitelist", AdminRemoveWhiteIP)
	}
}

func AdminDashboard(c *gin.Context) {
	days, _ := strconv.Atoi(c.DefaultQuery("days", "14"))
	stats, err := db.GetDashboardStats(days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, stats)
}

func AdminListPulls(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	list, total, err := db.ListPullSessions(db.PullListFilter{
		IP:       c.Query("ip"),
		Image:    c.Query("image"),
		Category: c.Query("category"),
		Registry: c.Query("registry"),
		Status:   c.Query("status"),
		From:     c.Query("from"),
		To:       c.Query("to"),
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": list, "total": total, "page": page, "page_size": pageSize})
}

func AdminGetPull(c *gin.Context) {
	s, events, err := db.GetPullSession(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"session": s, "events": events})
}

func AdminListImages(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	list, total, err := db.ListImageStats(c.Query("image"), c.Query("category"), c.Query("registry"), page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": list, "total": total, "page": page, "page_size": pageSize})
}

func AdminListIPs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	list, total, err := db.ListIPStats(c.Query("ip"), page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": list, "total": total, "page": page, "page_size": pageSize})
}

func AdminListUsers(c *gin.Context) {
	list, err := db.ListUsers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": list})
}

type createUserRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	Role     string `json:"role"`
}

func AdminCreateUser(c *gin.Context) {
	var req createUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数无效"})
		return
	}
	user, err := db.CreateUser(req.Username, req.Password, req.Role)
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

type updateUserRequest struct {
	Username       *string `json:"username"`
	Role           *string `json:"role"`
	Password       *string `json:"password"`
	DailyPullLimit *int    `json:"daily_pull_limit"`
}

func AdminUpdateUser(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的用户 ID", "code": "BAD_REQUEST"})
		return
	}
	var req updateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数无效", "code": "BAD_REQUEST"})
		return
	}
	if req.Username != nil {
		if err := db.UpdateUsername(id, *req.Username); err != nil {
			if err == db.ErrUserExists {
				c.JSON(http.StatusConflict, gin.H{"error": "用户名已存在", "code": "USER_EXISTS"})
				return
			}
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "code": "BAD_REQUEST"})
			return
		}
	}
	if req.Role != nil {
		if err := db.UpdateUserRole(id, *req.Role); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "code": "BAD_REQUEST"})
			return
		}
	}
	if req.Password != nil {
		if err := db.UpdatePassword(id, *req.Password); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "code": "BAD_REQUEST"})
			return
		}
		_ = db.DeleteUserSessions(id)
	}
	if req.DailyPullLimit != nil {
		if err := db.UpdateDailyPullLimit(id, *req.DailyPullLimit); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "code": "BAD_REQUEST"})
			return
		}
	}
	user, err := db.GetUserByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在", "code": "NOT_FOUND"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": user})
}

func AdminDeleteUser(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	me := currentUser(c)
	if me.ID == id {
		c.JSON(http.StatusBadRequest, gin.H{"error": "不能删除当前登录用户"})
		return
	}
	if err := db.DeleteUser(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func AdminGetSettings(c *gin.Context) {
	rl, sec, acc, adm, ps := db.GlobalRuntime.Snapshot()
	oauth := db.GlobalRuntime.GetOAuth()
	oauthView := oauth
	if oauthView.ClientSecret != "" {
		oauthView.ClientSecret = "********"
	}
	email := db.GlobalRuntime.GetEmail()
	emailView := email
	if emailView.Password != "" {
		emailView.Password = "********"
	}
	scheme := "http"
	if c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	host := c.Request.Host
	if xf := c.GetHeader("X-Forwarded-Host"); xf != "" {
		host = xf
	}
	c.JSON(http.StatusOK, gin.H{
		"rate_limit":         rl,
		"security":           sec,
		"access":             acc,
		"admin":              adm,
		"pull_session":       ps,
		"features":           db.GlobalRuntime.GetFeatures(),
		"registries":         db.GlobalRuntime.GetRegistries(),
		"site":               db.GlobalRuntime.GetSite(),
		"oauth":              oauthView,
		"email":              emailView,
		"oauth_redirect_url": scheme + "://" + host + "/api/admin/oauth/callback",
	})
}

func AdminPutSite(c *gin.Context) {
	var req db.SiteSettings
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数无效"})
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "站点名称不能为空"})
		return
	}
	if err := db.SetSetting(db.KeySite, req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	db.GlobalRuntime.Reload()
	c.JSON(http.StatusOK, gin.H{"site": db.GlobalRuntime.GetSite()})
}

func AdminPutOAuth(c *gin.Context) {
	var req db.OAuthSettings
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数无效"})
		return
	}
	prev := db.GlobalRuntime.GetOAuth()
	if req.ClientSecret == "" || req.ClientSecret == "********" {
		req.ClientSecret = prev.ClientSecret
	}
	if err := db.SetSetting(db.KeyOAuth, req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	db.GlobalRuntime.Reload()
	out := db.GlobalRuntime.GetOAuth()
	if out.ClientSecret != "" {
		out.ClientSecret = "********"
	}
	c.JSON(http.StatusOK, gin.H{"oauth": out})
}

func AdminPutFeatures(c *gin.Context) {
	var req db.FeatureToggles
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数无效"})
		return
	}
	if err := db.SetSetting(db.KeyFeatures, req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	db.GlobalRuntime.Reload()
	c.JSON(http.StatusOK, gin.H{"features": db.GlobalRuntime.GetFeatures()})
}

func AdminPutRegistries(c *gin.Context) {
	var req []db.RegistryToggle
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数无效"})
		return
	}
	if req == nil {
		req = []db.RegistryToggle{}
	}
	if err := db.SetSetting(db.KeyRegistries, req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	db.GlobalRuntime.Reload()
	c.JSON(http.StatusOK, gin.H{"registries": db.GlobalRuntime.GetRegistries()})
}

func AdminPutRateLimit(c *gin.Context) {
	var req db.RateLimitSettings
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数无效"})
		return
	}
	if req.RequestLimit < 1 || req.PeriodHours <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "限流参数无效"})
		return
	}
	if req.PullLimit < 0 {
		req.PullLimit = 0
	}
	if err := db.SetSetting(db.KeyRateLimit, req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	db.GlobalRuntime.Reload()
	db.ApplySecurityToLimiter()
	c.JSON(http.StatusOK, gin.H{"rate_limit": db.GlobalRuntime.GetRateLimit()})
}

func AdminPutSecurity(c *gin.Context) {
	var req db.SecuritySettings
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数无效"})
		return
	}
	if req.WhiteList == nil {
		req.WhiteList = []string{}
	}
	if req.BlackList == nil {
		req.BlackList = []string{}
	}
	if err := db.SetSetting(db.KeySecurity, req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	db.GlobalRuntime.Reload()
	db.ApplySecurityToLimiter()
	c.JSON(http.StatusOK, gin.H{"security": db.GlobalRuntime.GetSecurity()})
}

func AdminPutAccess(c *gin.Context) {
	var req db.AccessSettings
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数无效"})
		return
	}
	if req.WhiteList == nil {
		req.WhiteList = []string{}
	}
	if req.BlackList == nil {
		req.BlackList = []string{}
	}
	if err := db.SetSetting(db.KeyAccess, req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	db.GlobalRuntime.Reload()
	db.ApplyAccessToController()
	c.JSON(http.StatusOK, gin.H{"access": db.GlobalRuntime.GetAccess()})
}

func AdminPutAdmin(c *gin.Context) {
	var req db.AdminSettings
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数无效"})
		return
	}
	if req.FormRegisterEnabled {
		req.RegisterEnabled = true
	} else {
		req.RegisterEnabled = false
	}
	if err := db.SetSetting(db.KeyAdmin, req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	db.GlobalRuntime.Reload()
	c.JSON(http.StatusOK, gin.H{"admin": db.GlobalRuntime.GetAdmin()})
}

func AdminPutPullSession(c *gin.Context) {
	var req db.PullSessionSettings
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数无效"})
		return
	}
	if req.WindowMinutes < 1 {
		req.WindowMinutes = 15
	}
	if req.IdleMinutes < 1 {
		req.IdleMinutes = 30
	}
	if req.ManifestProbeSeconds < 15 {
		req.ManifestProbeSeconds = 60
	}
	if err := db.SetSetting(db.KeyPullSession, req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	db.GlobalRuntime.Reload()
	c.JSON(http.StatusOK, gin.H{"pull_session": db.GlobalRuntime.GetPullSession()})
}

type ipItemRequest struct {
	IP string `json:"ip" binding:"required"`
}

func normalizeIPEntry(ip string) string {
	return strings.TrimSpace(ip)
}

func listContains(list []string, item string) bool {
	for _, v := range list {
		if v == item {
			return true
		}
	}
	return false
}

func removeFromList(list []string, item string) []string {
	out := make([]string, 0, len(list))
	for _, v := range list {
		if v != item {
			out = append(out, v)
		}
	}
	return out
}

func AdminAddBlackIP(c *gin.Context) {
	var req ipItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数无效"})
		return
	}
	ip := normalizeIPEntry(req.IP)
	if ip == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "IP 无效"})
		return
	}
	sec := db.GlobalRuntime.GetSecurity()
	if !listContains(sec.BlackList, ip) {
		sec.BlackList = append(sec.BlackList, ip)
	}
	if err := db.SetSetting(db.KeySecurity, sec); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	db.GlobalRuntime.Reload()
	db.ApplySecurityToLimiter()
	c.JSON(http.StatusOK, gin.H{"security": db.GlobalRuntime.GetSecurity()})
}

func AdminRemoveBlackIP(c *gin.Context) {
	ip := normalizeIPEntry(c.Query("ip"))
	if ip == "" {
		var req ipItemRequest
		_ = c.ShouldBindJSON(&req)
		ip = normalizeIPEntry(req.IP)
	}
	if ip == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "IP 无效"})
		return
	}
	sec := db.GlobalRuntime.GetSecurity()
	sec.BlackList = removeFromList(sec.BlackList, ip)
	if err := db.SetSetting(db.KeySecurity, sec); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	db.GlobalRuntime.Reload()
	db.ApplySecurityToLimiter()
	c.JSON(http.StatusOK, gin.H{"security": db.GlobalRuntime.GetSecurity()})
}

func AdminAddWhiteIP(c *gin.Context) {
	var req ipItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数无效"})
		return
	}
	ip := normalizeIPEntry(req.IP)
	sec := db.GlobalRuntime.GetSecurity()
	if !listContains(sec.WhiteList, ip) {
		sec.WhiteList = append(sec.WhiteList, ip)
	}
	if err := db.SetSetting(db.KeySecurity, sec); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	db.GlobalRuntime.Reload()
	db.ApplySecurityToLimiter()
	c.JSON(http.StatusOK, gin.H{"security": db.GlobalRuntime.GetSecurity()})
}

func AdminRemoveWhiteIP(c *gin.Context) {
	ip := normalizeIPEntry(c.Query("ip"))
	if ip == "" {
		var req ipItemRequest
		_ = c.ShouldBindJSON(&req)
		ip = normalizeIPEntry(req.IP)
	}
	sec := db.GlobalRuntime.GetSecurity()
	sec.WhiteList = removeFromList(sec.WhiteList, ip)
	if err := db.SetSetting(db.KeySecurity, sec); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	db.GlobalRuntime.Reload()
	db.ApplySecurityToLimiter()
	c.JSON(http.StatusOK, gin.H{"security": db.GlobalRuntime.GetSecurity()})
}
