package handlers

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"hubproxy/config"
)

const (
	siteGateCookie = "qingyu_site_gate"
	siteGateTTL    = 7 * 24 * time.Hour
)

func siteGateCookieValue(key string) string {
	sum := sha256.Sum256([]byte("qingyu-site-gate:" + key))
	return hex.EncodeToString(sum[:])
}

func isDockerOrProxyPath(path string) bool {
	if path == "/v2" || path == "/v2/" || strings.HasPrefix(path, "/v2/") {
		return true
	}
	if path == "/token" || strings.HasPrefix(path, "/token/") {
		return true
	}
	if path == "/ready" {
		return true
	}
	// registry-mirrors: /{8-char-token}/v2...
	trim := strings.TrimPrefix(path, "/")
	parts := strings.SplitN(trim, "/", 3)
	if len(parts) >= 2 && len(parts[0]) == 8 && parts[1] == "v2" {
		ok := true
		for _, r := range parts[0] {
			if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	// GitHub / HF raw proxy is not a SPA path — allow through gate so pulls/downloads work
	// but we only skip gate for known non-browser API shapes. SPA paths still gated.
	return false
}

func isPublicSPAPath(path string) bool {
	return path == "/" ||
		path == "/images" ||
		path == "/search" ||
		path == "/favicon.ico" ||
		strings.HasPrefix(path, "/assets/") ||
		path == "/admin" ||
		strings.HasPrefix(path, "/admin/") ||
		strings.HasPrefix(path, "/api/")
}

// SiteGateMiddleware blocks browser UI when siteAccessKey is configured,
// until visitor unlocks via /gate/{key}. Docker pull paths are never blocked.
func SiteGateMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := config.GetConfig()
		key := strings.TrimSpace(cfg.Server.SiteAccessKey)
		if key == "" {
			c.Next()
			return
		}
		path := c.Request.URL.Path
		if isDockerOrProxyPath(path) {
			c.Next()
			return
		}
		// unlock endpoint always open
		if strings.HasPrefix(path, "/gate/") {
			c.Next()
			return
		}
		// only gate SPA + admin API (+ public search/image APIs used by frontend)
		if !isPublicSPAPath(path) {
			// other paths (GitHub proxy etc.) still allowed — pull/download
			c.Next()
			return
		}

		want := siteGateCookieValue(key)
		got, err := c.Cookie(siteGateCookie)
		if err == nil && subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1 {
			c.Next()
			return
		}

		// Blocked: return minimal response, no SPA shell (prevents indexing/token leak via HTML)
		c.Header("X-Robots-Tag", "noindex, nofollow, noarchive")
		c.Header("Cache-Control", "no-store")
		if strings.HasPrefix(path, "/api/") {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "站点未解锁，请使用访问密钥打开 /gate/{密钥}",
				"code":  "SITE_LOCKED",
			})
			return
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"error": "forbidden",
			"code":  "SITE_LOCKED",
		})
	}
}

// SiteGateUnlock handles GET /gate/:key — sets cookie and redirects to home.
func SiteGateUnlock(c *gin.Context) {
	cfg := config.GetConfig()
	key := strings.TrimSpace(cfg.Server.SiteAccessKey)
	if key == "" {
		c.Redirect(http.StatusFound, "/")
		return
	}
	provided := c.Param("key")
	if subtle.ConstantTimeCompare([]byte(provided), []byte(key)) != 1 {
		c.Header("X-Robots-Tag", "noindex, nofollow")
		c.String(http.StatusNotFound, "not found")
		return
	}
	secure := c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https")
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(siteGateCookie, siteGateCookieValue(key), int(siteGateTTL.Seconds()), "/", "", secure, true)
	c.Header("X-Robots-Tag", "noindex, nofollow")
	c.Redirect(http.StatusFound, "/")
}
