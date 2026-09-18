package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"hubproxy/db"
)

const (
	ctxAccessUserID = "access_user_id"
)

// NormalizeMirrorTokenPath converts registry-mirrors path form:
//
//	/{token}/v2/...  →  /v2/{token}/...
//
// so the rest of the pipeline can share one code path.
// Supports daemon.json: "registry-mirrors": ["https://host/TOKEN"]
func NormalizeMirrorTokenPath() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		// /Ab12Cd34/v2  or /Ab12Cd34/v2/...
		trim := strings.TrimPrefix(path, "/")
		parts := strings.SplitN(trim, "/", 3)
		if len(parts) >= 2 && db.IsAccessTokenFormat(parts[0]) && parts[1] == "v2" {
			tok := parts[0]
			rest := ""
			if len(parts) == 3 {
				rest = parts[2]
			}
			if rest == "" {
				c.Request.URL.Path = "/v2/" + tok + "/"
			} else {
				c.Request.URL.Path = "/v2/" + tok + "/" + rest
			}
		}
		// /Ab12Cd34/token  → /token (auth with mirror prefix)
		if len(parts) >= 2 && db.IsAccessTokenFormat(parts[0]) && parts[1] == "token" {
			if at, err := db.GetActiveToken(parts[0]); err == nil {
				c.Set(ctxAccessUserID, at.UserID)
			}
			if len(parts) == 2 {
				c.Request.URL.Path = "/token"
			} else {
				c.Request.URL.Path = "/token/" + strings.Join(parts[2:], "/")
			}
		}
		c.Next()
	}
}

// ProxyDockerRegistryMirrorPrefix handles /:token/v2/* after path params are set.
func ProxyDockerRegistryMirrorPrefix(c *gin.Context) {
	tok := c.Param("token")
	if !db.IsAccessTokenFormat(tok) {
		c.Status(http.StatusNotFound)
		return
	}
	rest := c.Param("filepath")
	if rest == "" || rest == "/" {
		c.Request.URL.Path = "/v2/" + tok + "/"
	} else {
		if !strings.HasPrefix(rest, "/") {
			rest = "/" + rest
		}
		c.Request.URL.Path = "/v2/" + tok + rest
	}
	ProxyDockerRegistryGin(c)
}

// stripUserAccessToken rewrites /v2/{token}/... for user-scoped pulls.
// Styles supported:
//  1. Explicit: docker pull host/TOKEN/nginx → /v2/TOKEN/library/nginx/...
//  2. Mirror:   registry-mirrors https://host/TOKEN → /TOKEN/v2/library/nginx/... (normalized first)
func stripUserAccessToken(c *gin.Context) (denied string) {
	feat := db.GlobalRuntime.GetFeatures()
	path := c.Request.URL.Path

	// /v2/ or bare ping: always allow for registry discovery
	if path == "/v2/" || path == "/v2" {
		return ""
	}

	if strings.HasPrefix(path, "/v2/") {
		rest := strings.TrimPrefix(path, "/v2/")
		parts := strings.SplitN(rest, "/", 2)
		if len(parts) >= 1 && db.IsAccessTokenFormat(parts[0]) {
			tok := parts[0]
			if _, denied := authenticateAccessTokenPath(c, tok); denied != "" {
				return denied
			}
			// rewrite path without token segment
			if len(parts) == 1 || parts[1] == "" {
				c.Request.URL.Path = "/v2/"
			} else {
				c.Request.URL.Path = "/v2/" + parts[1]
			}
			return ""
		}
		// Public mirror disabled → must use personal token path
		if !feat.AllowPublicDockerPull() {
			return "公共镜像已关闭，请使用个人令牌路径：docker pull 域名/令牌/镜像 或 registry-mirrors: https://域名/令牌"
		}
		return ""
	}

	return ""
}

func authenticateAccessTokenPath(c *gin.Context, tok string) (userID int64, denied string) {
	if !db.IsAccessTokenFormat(tok) {
		return 0, "访问令牌无效或已重置"
	}
	at, err := db.GetActiveToken(tok)
	if err != nil {
		return 0, "访问令牌无效或已重置"
	}
	ok, err := db.CheckUserIPAllowed(at.UserID, c.ClientIP())
	if err != nil || !ok {
		return 0, "当前 IP 不在该用户白名单内"
	}
	c.Set(ctxAccessUserID, at.UserID)
	return at.UserID, ""
}

func accessUserFromContext(c *gin.Context) (userID int64) {
	if v, ok := c.Get(ctxAccessUserID); ok {
		if id, ok := v.(int64); ok {
			userID = id
		}
	}
	return
}

func denyDocker(c *gin.Context, status int, msg string) {
	c.Header("Docker-Distribution-API-Version", "registry/2.0")
	// Docker clients expect plain text; also set Docker-compatible error when possible
	c.String(status, msg)
}

func denyDockerAuth(c *gin.Context, msg string) {
	c.Header("Docker-Distribution-API-Version", "registry/2.0")
	c.Header("WWW-Authenticate", `Bearer realm="token",error="invalid_token"`)
	c.String(http.StatusUnauthorized, msg)
}

func denyDockerForbidden(c *gin.Context, msg string) {
	c.Header("Docker-Distribution-API-Version", "registry/2.0")
	c.String(http.StatusForbidden, msg)
}

func denyDockerTooMany(c *gin.Context, msg string) {
	c.Header("Docker-Distribution-API-Version", "registry/2.0")
	c.Header("Retry-After", "3600")
	c.String(http.StatusTooManyRequests, msg)
}

// Feature guard helpers
func ensureDockerHubEnabled(c *gin.Context) bool {
	if !db.GlobalRuntime.GetFeatures().DockerHub {
		denyDocker(c, http.StatusForbidden, "Docker Hub 加速已关闭")
		return false
	}
	return true
}

func ensureRegistryEnabled(domain string) (bool, string) {
	if reg, ok := db.GlobalRuntime.GetRegistry(domain); ok {
		if !reg.Enabled {
			return false, fmt.Sprintf("Registry %s 已关闭", domain)
		}
		return true, ""
	}
	// unknown registry not in list
	return false, fmt.Sprintf("Registry %s 未配置", domain)
}
