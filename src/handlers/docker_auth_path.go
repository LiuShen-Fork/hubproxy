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
	ctxAccessToken  = "access_token"
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
			c.Set(ctxAccessToken, parts[0])
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
func stripUserAccessToken(c *gin.Context) (userID int64, token string, denied string) {
	feat := db.GlobalRuntime.GetFeatures()
	path := c.Request.URL.Path

	// /v2/ or bare ping: always allow for registry discovery
	if path == "/v2/" || path == "/v2" {
		return 0, "", ""
	}

	if strings.HasPrefix(path, "/v2/") {
		rest := strings.TrimPrefix(path, "/v2/")
		parts := strings.SplitN(rest, "/", 2)
		if len(parts) >= 1 && db.IsAccessTokenFormat(parts[0]) {
			tok := parts[0]
			at, err := db.GetActiveToken(tok)
			if err != nil {
				return 0, "", "访问令牌无效或已重置"
			}
			ok, err := db.CheckUserIPAllowed(at.UserID, c.ClientIP())
			if err != nil || !ok {
				return 0, "", "当前 IP 不在该用户白名单内"
			}
			// rewrite path without token segment
			if len(parts) == 1 || parts[1] == "" {
				c.Request.URL.Path = "/v2/"
			} else {
				c.Request.URL.Path = "/v2/" + parts[1]
			}
			c.Set(ctxAccessUserID, at.UserID)
			c.Set(ctxAccessToken, tok)
			return at.UserID, tok, ""
		}
		if feat.RequireUserToken {
			return 0, "", "请使用个人访问路径：docker pull 域名/令牌/镜像 或配置 registry-mirrors 为 https://域名/令牌"
		}
		return 0, "", ""
	}

	return 0, "", ""
}

func accessUserFromContext(c *gin.Context) (userID int64, token string) {
	if v, ok := c.Get(ctxAccessUserID); ok {
		if id, ok := v.(int64); ok {
			userID = id
		}
	}
	if v, ok := c.Get(ctxAccessToken); ok {
		if t, ok := v.(string); ok {
			token = t
		}
	}
	return
}

func denyDocker(c *gin.Context, status int, msg string) {
	c.Header("Docker-Distribution-API-Version", "registry/2.0")
	c.String(status, msg)
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
