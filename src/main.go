package main

import (
	"embed"
	"fmt"
	"log"
	"mime"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"hubproxy/config"
	"hubproxy/db"
	"hubproxy/handlers"
	"hubproxy/utils"
)

//go:embed all:dist
var staticFiles embed.FS

var (
	globalLimiter    *utils.IPRateLimiter
	serviceStartTime = time.Now()
)

var Version = "dev"

func init() {
	for ext, typ := range map[string]string{
		".js":    "application/javascript; charset=utf-8",
		".mjs":   "application/javascript; charset=utf-8",
		".woff":  "font/woff",
		".woff2": "font/woff2",
		".map":   "application/json",
	} {
		_ = mime.AddExtensionType(ext, typ)
	}
}

func contentTypeFor(filename string) string {
	if ct := mime.TypeByExtension(path.Ext(filename)); ct != "" {
		return ct
	}
	return "application/octet-stream"
}

func serveEmbedFile(c *gin.Context, filename string) {
	data, err := staticFiles.ReadFile(filename)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	c.Data(http.StatusOK, contentTypeFor(filename), data)
}

func serveSPA(c *gin.Context) {
	// prevent search engines / scrapers from indexing console shell
	c.Header("X-Robots-Tag", "noindex, nofollow, noarchive")
	c.Header("Referrer-Policy", "no-referrer")
	serveEmbedFile(c, "dist/index.html")
}

func registerFrontendRoutes(router *gin.Engine, enabled bool) {
	if !enabled {
		notFound := func(c *gin.Context) { c.Status(http.StatusNotFound) }
		router.GET("/", notFound)
		router.GET("/images", notFound)
		router.GET("/search", notFound)
		router.GET("/admin", notFound)
		router.GET("/admin/*path", notFound)
		router.GET("/assets/*filepath", notFound)
		router.GET("/favicon.ico", notFound)
		return
	}

	router.GET("/", serveSPA)
	router.GET("/images", serveSPA)
	router.GET("/search", serveSPA)
	router.GET("/admin", serveSPA)
	router.GET("/admin/*path", serveSPA)
	router.GET("/favicon.ico", func(c *gin.Context) {
		serveEmbedFile(c, "dist/favicon.ico")
	})
	router.GET("/assets/*filepath", func(c *gin.Context) {
		filepath := strings.TrimPrefix(c.Param("filepath"), "/")
		if filepath == "" || strings.Contains(filepath, "..") {
			c.Status(http.StatusNotFound)
			return
		}
		serveEmbedFile(c, path.Join("dist/assets", filepath))
	})
}

func buildRouter(cfg *config.AppConfig) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()
	utils.ConfigureTrustedProxies(router)

	router.Use(gin.CustomRecovery(func(c *gin.Context, recovered interface{}) {
		log.Printf("Panic 已恢复: %v", recovered)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Internal server error",
			"code":  "INTERNAL_ERROR",
		})
	}))

	// Block API paths from ever falling into GitHub proxy (must be early)
	router.Use(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.Set("api_request", true)
		}
		c.Next()
		// If an /api request ended with no response written and not aborted by a handler,
		// Gin will still hit NoRoute for unmatched routes — handled below.
	})

	router.Use(handlers.SecurityHeadersMiddleware())
	router.Use(utils.RateLimitMiddleware(globalLimiter))
	// registry-mirrors: https://host/TOKEN → /TOKEN/v2/... rewrite
	router.Use(handlers.NormalizeMirrorTokenPath())
	// site access key gate (UI only; docker pulls never blocked)
	router.Use(handlers.SiteGateMiddleware())

	initHealthRoutes(router)
	router.GET("/gate/:key", handlers.SiteGateUnlock)
	handlers.InitImageTarRoutes(router)
	handlers.RegisterAdminRoutes(router)
	registerFrontendRoutes(router, cfg.Server.EnableFrontend)
	handlers.RegisterSearchRoute(router)

	router.Any("/token", handlers.ProxyDockerAuthGin)
	router.Any("/token/*path", handlers.ProxyDockerAuthGin)
	router.Any("/v2/*path", handlers.ProxyDockerRegistryGin)
	// registry-mirrors path prefix form (explicit route; middleware also normalizes)
	router.Any("/:token/v2", handlers.ProxyDockerRegistryMirrorPrefix)
	router.Any("/:token/v2/*filepath", handlers.ProxyDockerRegistryMirrorPrefix)

	// API 未匹配路由返回 JSON，避免落入 GitHub 代理返回「无效输入」
	router.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{
				"error":   "接口不存在。若刚升级，请完全停止旧进程后用最新代码重启（go run .）",
				"code":    "NOT_FOUND",
				"path":    path,
				"version": Version,
			})
			return
		}
		handlers.GitHubProxyHandler(c)
	})

	return router
}

func wireRuntimeCallbacks() {
	db.OnAccessUpdated = func(access db.AccessSettings) {
		utils.GlobalAccessController.SetAccessLists(access.WhiteList, access.BlackList)
		utils.ApplyProxyEnv(access.Proxy)
	}
	db.OnSecurityUpdated = func(sec db.SecuritySettings, rl db.RateLimitSettings) {
		if globalLimiter != nil {
			globalLimiter.UpdateSecurity(sec.WhiteList, sec.BlackList, rl.RequestLimit, rl.PeriodHours)
		}
	}
	db.OnProxyUpdated = utils.ApplyProxyEnv
}

func startMaintenanceJobs() {
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			_ = db.CleanupExpiredSessions()
			_ = db.ExpireIdlePullSessions()
		}
	}()
}

func main() {
	if err := config.LoadConfig(); err != nil {
		fmt.Printf("配置加载失败: %v\n", err)
		return
	}

	cfg := config.GetConfig()
	if err := db.Init(cfg.Server.DatabasePath); err != nil {
		fmt.Printf("数据库初始化失败: %v\n", err)
		return
	}
	defer db.Close()

	if err := db.EnsureDefaultAdmin(); err != nil {
		fmt.Printf("初始化管理员失败: %v\n", err)
		return
	}
	if err := db.SeedFromConfig(cfg); err != nil {
		fmt.Printf("初始化设置失败: %v\n", err)
		return
	}

	wireRuntimeCallbacks()
	// Apply SQLite-backed security/access (seeded from config.toml on first run)
	db.ApplyAccessToController()
	// HTTP clients pick up proxy env set by ApplyAccessToController
	utils.InitHTTPClients()
	globalLimiter = utils.InitGlobalLimiter()
	db.ApplySecurityToLimiter()
	handlers.InitDockerProxy()
	handlers.InitImageStreamer()
	handlers.InitDebouncer()
	startMaintenanceJobs()

	router := buildRouter(cfg)

	rl := db.GlobalRuntime.GetRateLimit()
	fmt.Printf("HubProxy 启动成功\n")
	fmt.Printf("监听地址: %s:%d\n", cfg.Server.Host, cfg.Server.Port)
	fmt.Printf("限流配置: %d请求/%g小时, 拉取会话上限 %d\n", rl.RequestLimit, rl.PeriodHours, rl.PullLimit)
	fmt.Printf("管理后台: http://%s:%d/admin （默认 admin / admin12346，首次请改密）\n",
		func() string {
			if cfg.Server.Host == "0.0.0.0" {
				return "127.0.0.1"
			}
			return cfg.Server.Host
		}(), cfg.Server.Port)
	if cfg.Server.EnableH2C {
		fmt.Printf("H2c: 已启用\n")
	}
	fmt.Printf("版本号: %s\n", Version)
	fmt.Printf("清羽镜像 · 清羽飞扬自建多源镜像（仅供自用）\n")
	if k := strings.TrimSpace(cfg.Server.SiteAccessKey); k != "" {
		fmt.Printf("站点访问密钥已启用，浏览器请先打开: /gate/%s\n", k)
	}

	server := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 30 * time.Minute,
		IdleTimeout:  120 * time.Second,
	}

	if cfg.Server.EnableH2C {
		server.Handler = h2c.NewHandler(router, &http2.Server{
			MaxConcurrentStreams:         250,
			IdleTimeout:                  300 * time.Second,
			MaxReadFrameSize:             4 << 20,
			MaxUploadBufferPerConnection: 8 << 20,
			MaxUploadBufferPerStream:     2 << 20,
		})
	} else {
		server.Handler = router
	}

	if err := server.ListenAndServe(); err != nil {
		fmt.Printf("启动服务失败: %v\n", err)
	}
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%d秒", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%d分钟%d秒", int(d.Minutes()), int(d.Seconds())%60)
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%d小时%d分钟", int(d.Hours()), int(d.Minutes())%60)
	}

	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	return fmt.Sprintf("%d天%d小时", days, hours)
}

func getUptimeInfo() (time.Duration, float64, string) {
	uptime := time.Since(serviceStartTime)
	return uptime, uptime.Seconds(), formatDuration(uptime)
}

func initHealthRoutes(router *gin.Engine) {
	router.GET("/ready", func(c *gin.Context) {
		_, uptimeSec, uptimeHuman := getUptimeInfo()
		c.JSON(http.StatusOK, gin.H{
			"ready":           true,
			"service":         "qingyu-mirror",
			"name":            "清羽镜像",
			"version":         Version,
			"features":        []string{"user_token", "user_console", "feature_toggles", "mirror_path_prefix"},
			"start_time_unix": serviceStartTime.Unix(),
			"uptime_sec":      uptimeSec,
			"uptime_human":    uptimeHuman,
		})
	})
}
