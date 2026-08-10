package handlers

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"hubproxy/config"
	"hubproxy/utils"
)

var (
	// GitHub URL匹配正则表达式
	githubExps = []*regexp.Regexp{
		regexp.MustCompile(`^(?:https?://)?github\.com/([^/]+)/([^/]+)/(?:releases|archive)/.*`),
		regexp.MustCompile(`^(?:https?://)?github\.com/([^/]+)/([^/]+)/(?:blob|raw)/.*`),
		regexp.MustCompile(`^(?:https?://)?github\.com/([^/]+)/([^/]+)/(?:info|git-).*`),
		regexp.MustCompile(`^(?:https?://)?raw\.github(?:usercontent|)\.com/([^/]+)/([^/]+)/.+?/.+`),
		regexp.MustCompile(`^(?:https?://)?gist\.(?:githubusercontent|github)\.com/([^/]+)/([^/]+).*`),
		regexp.MustCompile(`^(?:https?://)?api\.github\.com/repos/([^/]+)/([^/]+)/.*`),
		regexp.MustCompile(`^(?:https?://)?huggingface\.co(?:/spaces)?/([^/]+)/(.+)`),
		regexp.MustCompile(`^(?:https?://)?cdn-lfs\.hf\.co(?:/spaces)?/([^/]+)/([^/]+)(?:/(.*))?`),
		regexp.MustCompile(`^(?:https?://)?(github|opengraph)\.githubassets\.com/([^/]+)/.+?`),
	}
)

// 全局变量：被阻止的内容类型
var blockedContentTypes = map[string]bool{
	"text/html":             true,
	"application/xhtml+xml": true,
	"text/xml":              true,
	"application/xml":       true,
}

// GitHubProxyHandler GitHub代理处理器
func GitHubProxyHandler(c *gin.Context) {
	rawPath := strings.TrimPrefix(c.Request.URL.RequestURI(), "/")

	for strings.HasPrefix(rawPath, "/") {
		rawPath = strings.TrimPrefix(rawPath, "/")
	}
	proxyAcceleratedRawPath(c, rawPath, "")
}

// CheckGitHubURL 检查URL是否匹配GitHub模式
func CheckGitHubURL(u string) []string {
	for _, exp := range githubExps {
		if matches := exp.FindStringSubmatch(u); matches != nil {
			return matches[1:]
		}
	}
	return nil
}

// ProxyGitHubRequest 代理GitHub请求
func ProxyGitHubRequest(c *gin.Context, u string) {
	proxyGitHubWithRedirect(c, u, 0)
}

// proxyGitHubWithRedirect 带重定向的GitHub代理请求
func proxyGitHubWithRedirect(c *gin.Context, u string, redirectCount int) {
	const maxRedirects = 20
	if redirectCount > maxRedirects {
		c.String(http.StatusLoopDetected, "重定向次数过多，可能存在循环重定向")
		return
	}

	req, err := http.NewRequest(c.Request.Method, u, c.Request.Body)
	if err != nil {
		c.String(http.StatusInternalServerError, fmt.Sprintf("server error %v", err))
		return
	}

	// 复制请求头
	for key, values := range c.Request.Header {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
	req.Header.Del("Host")

	resp, err := utils.GetGlobalHTTPClient().Do(req)
	if err != nil {
		c.String(http.StatusInternalServerError, fmt.Sprintf("server error %v", err))
		return
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fmt.Printf("关闭响应体失败: %v\n", err)
		}
	}()

	// 检查并处理被阻止的内容类型
	if c.Request.Method == "GET" {
		if contentType := resp.Header.Get("Content-Type"); blockedContentTypes[strings.ToLower(strings.Split(contentType, ";")[0])] {
			c.JSON(http.StatusForbidden, map[string]string{
				"error":   "Content type not allowed",
				"message": "检测到网页类型，本服务不支持加速网页，请检查您的链接是否正确。",
			})
			return
		}
	}

	// 检查文件大小限制
	cfg := config.GetConfig()
	if contentLength := resp.Header.Get("Content-Length"); contentLength != "" {
		if size, err := strconv.ParseInt(contentLength, 10, 64); err == nil && size > cfg.Server.FileSize {
			c.String(http.StatusRequestEntityTooLarge,
				fmt.Sprintf("文件过大，限制大小: %d MB", cfg.Server.FileSize/(1024*1024)))
			return
		}
	}

	// 清理安全相关的头
	resp.Header.Del("Content-Security-Policy")
	resp.Header.Del("Referrer-Policy")
	resp.Header.Del("Strict-Transport-Security")

	realHost := contentProxyExternalBase(c)

	// 处理.sh和.ps1文件的智能处理
	if strings.HasSuffix(strings.ToLower(u), ".sh") || strings.HasSuffix(strings.ToLower(u), ".ps1") {
		isGzipCompressed := resp.Header.Get("Content-Encoding") == "gzip"

		processedBody, processedSize, err := utils.ProcessSmart(resp.Body, isGzipCompressed, realHost)
		if err != nil {
			fmt.Printf("脚本处理失败: %v\n", err)
			c.String(http.StatusBadGateway, "Script processing failed: %v", err)
			return
		}

		// 智能设置响应头
		if processedSize > 0 {
			resp.Header.Del("Content-Length")
			resp.Header.Del("Content-Encoding")
			resp.Header.Set("Transfer-Encoding", "chunked")
		}

		// 复制其他响应头
		for key, values := range resp.Header {
			for _, value := range values {
				c.Header(key, value)
			}
		}

		// 处理重定向
		if location := resp.Header.Get("Location"); location != "" {
			if CheckGitHubURL(location) != nil {
				c.Header("Location", contentProxyRedirectLocation(c, location))
			} else {
				proxyGitHubWithRedirect(c, location, redirectCount+1)
				return
			}
		}

		c.Status(resp.StatusCode)

		// 输出处理后的内容
		n, err := io.Copy(c.Writer, processedBody)
		if n == 0 && processedSize > 0 {
			n = processedSize
		}
		recordContentBytes(c, u, n, resp.StatusCode)
		if err != nil {
			return
		}
	} else {
		// 复制响应头
		for key, values := range resp.Header {
			for _, value := range values {
				c.Header(key, value)
			}
		}

		// 处理重定向
		if location := resp.Header.Get("Location"); location != "" {
			if CheckGitHubURL(location) != nil {
				c.Header("Location", contentProxyRedirectLocation(c, location))
			} else {
				proxyGitHubWithRedirect(c, location, redirectCount+1)
				return
			}
		}

		c.Status(resp.StatusCode)

		// 直接流式转发
		n, err := io.Copy(c.Writer, resp.Body)
		recordContentBytes(c, u, n, resp.StatusCode)
		if err != nil {
			fmt.Printf("转发响应体失败: %v\n", err)
		}
	}
}
