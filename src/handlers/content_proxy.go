package handlers

import (
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
	"hubproxy/db"
	"hubproxy/utils"
)

const ctxContentProxyPrefix = "content_proxy_prefix"

const (
	contentKindGitHub      = "github"
	contentKindHuggingFace = "huggingface"
)

func ProxyGitHubTokenPath(c *gin.Context) {
	proxyTokenContentPath(c, contentKindGitHub, "gh")
}

func ProxyHuggingFaceTokenPath(c *gin.Context) {
	proxyTokenContentPath(c, contentKindHuggingFace, "hf")
}

func ProxyGitHubPublicPath(c *gin.Context) {
	c.Set(ctxContentProxyPrefix, "/gh")
	proxyAcceleratedRawPath(c, contentPathParamWithQuery(c), contentKindGitHub)
}

func ProxyHuggingFacePublicPath(c *gin.Context) {
	c.Set(ctxContentProxyPrefix, "/hf")
	proxyAcceleratedRawPath(c, contentPathParamWithQuery(c), contentKindHuggingFace)
}

func proxyTokenContentPath(c *gin.Context, kind, prefix string) {
	tok := c.Param("token")
	if _, denied := authenticateAccessTokenPath(c, tok); denied != "" {
		status := http.StatusUnauthorized
		if strings.Contains(denied, "白名单") {
			status = http.StatusForbidden
		}
		c.String(status, denied)
		return
	}
	c.Set(ctxContentProxyPrefix, "/"+tok+"/"+prefix)
	proxyAcceleratedRawPath(c, contentPathParamWithQuery(c), kind)
}

func contentPathParamWithQuery(c *gin.Context) string {
	rawPath := strings.TrimPrefix(c.Param("path"), "/")
	if c.Request.URL.RawQuery != "" {
		if rawPath == "" {
			rawPath = "?" + c.Request.URL.RawQuery
		} else {
			rawPath += "?" + c.Request.URL.RawQuery
		}
	}
	return rawPath
}

func proxyAcceleratedRawPath(c *gin.Context, rawPath, requiredKind string) {
	rawPath = strings.TrimLeft(rawPath, "/")
	if rawPath == "" {
		c.String(http.StatusBadRequest, "缺少加速地址")
		return
	}

	targetURL := normalizeAcceleratedURL(rawPath)
	matches := CheckGitHubURL(targetURL)
	if matches == nil {
		c.String(http.StatusForbidden, "无效输入")
		return
	}

	kind := classifyAcceleratedURL(targetURL)
	if requiredKind != "" && kind != requiredKind {
		c.String(http.StatusForbidden, "地址类型与路径不匹配")
		return
	}

	feat := db.GlobalRuntime.GetFeatures()
	if kind == contentKindHuggingFace {
		if !feat.HuggingFace {
			c.String(http.StatusForbidden, "Hugging Face 加速已关闭")
			return
		}
	} else if !feat.GitHub {
		c.String(http.StatusForbidden, "GitHub 加速已关闭")
		return
	}

	if accessUserFromContext(c) == 0 && !feat.AllowPublicDockerPull() {
		c.String(http.StatusForbidden, "公共加速已关闭，请使用个人令牌路径")
		return
	}

	if allowed, reason := utils.GlobalAccessController.CheckGitHubAccess(matches); !allowed {
		var repoPath string
		if len(matches) >= 2 {
			username := matches[0]
			repoName := strings.TrimSuffix(matches[1], ".git")
			repoPath = username + "/" + repoName
		}
		fmt.Printf("内容资源 %s 访问被拒绝: %s\n", repoPath, reason)
		c.String(http.StatusForbidden, reason)
		return
	}

	// Convert GitHub blob links to raw before tracking/proxying.
	if githubExps[1].MatchString(targetURL) {
		targetURL = strings.Replace(targetURL, "/blob/", "/raw/", 1)
	}

	registry, imageName, reference := contentTrackingFields(targetURL, matches)
	if _, reason := trackContentPull(c, imageName, registry, reference, kind); reason != "" {
		c.String(http.StatusTooManyRequests, reason)
		return
	}

	ProxyGitHubRequest(c, targetURL)
}

func normalizeAcceleratedURL(rawPath string) string {
	if !strings.HasPrefix(rawPath, "https://") {
		if strings.HasPrefix(rawPath, "http://") {
			rawPath = strings.TrimPrefix(rawPath, "http://")
		} else if strings.HasPrefix(rawPath, "http:/") || strings.HasPrefix(rawPath, "https:/") {
			rawPath = strings.Replace(rawPath, "http:/", "", 1)
			rawPath = strings.Replace(rawPath, "https:/", "", 1)
		}
		rawPath = "https://" + rawPath
	}
	return rawPath
}

func classifyAcceleratedURL(rawURL string) string {
	if isHuggingFaceURL(rawURL) {
		return contentKindHuggingFace
	}
	return contentKindGitHub
}

func isHuggingFaceURL(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return strings.Contains(rawURL, "huggingface.co") || strings.Contains(rawURL, "cdn-lfs.hf.co")
	}
	host := strings.ToLower(parsed.Hostname())
	return host == "huggingface.co" || host == "cdn-lfs.hf.co"
}

func contentTrackingFields(rawURL string, matches []string) (registry, imageName, reference string) {
	parsed, err := url.Parse(rawURL)
	if err == nil {
		registry = strings.ToLower(parsed.Hostname())
		cleanPath := strings.TrimPrefix(parsed.Path, "/")
		if cleanPath == "" {
			cleanPath = registry
		}
		imageName = registry + "/" + cleanPath
		if parsed.RawQuery != "" {
			imageName += "?" + parsed.RawQuery
		}
		reference = contentBaseName(parsed.Path)
	}
	if registry == "" {
		registry = classifyAcceleratedURL(rawURL)
	}
	if imageName == "" {
		imageName = rawURL
	}
	if reference == "" && len(matches) >= 2 {
		reference = matches[0] + "/" + strings.TrimSuffix(matches[1], ".git")
	}
	if reference == "" {
		reference = "file"
	}
	return
}

func contentBaseName(p string) string {
	p = strings.TrimSuffix(p, "/")
	if p == "" {
		return ""
	}
	base := path.Base(p)
	if base == "." || base == "/" {
		return ""
	}
	return base
}

func contentProxyExternalBase(c *gin.Context) string {
	scheme := "http"
	if c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	host := c.Request.Host
	if xf := c.GetHeader("X-Forwarded-Host"); xf != "" {
		host = xf
	}
	base := scheme + "://" + host
	if prefix, ok := c.Get(ctxContentProxyPrefix); ok {
		if p, ok := prefix.(string); ok && p != "" {
			base += p
		}
	}
	return base
}

func contentProxyRedirectLocation(c *gin.Context, location string) string {
	location = strings.TrimLeft(location, "/")
	if prefix, ok := c.Get(ctxContentProxyPrefix); ok {
		if p, ok := prefix.(string); ok && p != "" {
			return p + "/" + location
		}
	}
	return "/" + location
}
