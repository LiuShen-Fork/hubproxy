package utils

import (
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"hubproxy/config"
)

var (
	globalHTTPClient *http.Client
	searchHTTPClient *http.Client
)

// ApplyProxyEnv updates outbound proxy environment used by HTTP transports.
func ApplyProxyEnv(proxy string) {
	proxy = strings.TrimSpace(proxy)
	if proxy == "" {
		os.Unsetenv("HTTP_PROXY")
		os.Unsetenv("HTTPS_PROXY")
		os.Unsetenv("http_proxy")
		os.Unsetenv("https_proxy")
		return
	}
	os.Setenv("HTTP_PROXY", proxy)
	os.Setenv("HTTPS_PROXY", proxy)
}

// InitHTTPClients 初始化HTTP客户端（出站代理由 ApplyProxyEnv / 后台配置注入）
func InitHTTPClients() {
	_ = config.GetConfig()

	globalHTTPClient = &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			MaxIdleConns:          1000,
			MaxIdleConnsPerHost:   1000,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			ResponseHeaderTimeout: 300 * time.Second,
		},
	}

	searchHTTPClient = &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   5 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
			TLSHandshakeTimeout: 5 * time.Second,
			DisableCompression:  false,
		},
	}
}

// GetGlobalHTTPClient 获取全局HTTP客户端
func GetGlobalHTTPClient() *http.Client {
	return globalHTTPClient
}

// GetSearchHTTPClient 获取搜索HTTP客户端
func GetSearchHTTPClient() *http.Client {
	return searchHTTPClient
}
