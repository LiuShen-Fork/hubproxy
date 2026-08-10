package utils

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
	"hubproxy/config"
)

const (
	CleanupInterval = 20 * time.Minute
	MaxIPCacheSize  = 10000
)

// 可信反代
var trustedProxyCIDRs = []string{
	"127.0.0.0/8",
	"10.0.0.0/8",
	"172.16.0.0/12",
	"192.168.0.0/16",
}

// ConfigureTrustedProxies 可信反代
func ConfigureTrustedProxies(router *gin.Engine) {
	_ = router.SetTrustedProxies(trustedProxyCIDRs)
}

// IPRateLimiter IP限流器结构体
type IPRateLimiter struct {
	ips              map[string]*rateLimiterEntry
	mu               *sync.RWMutex
	r                rate.Limit
	b                int
	whitelist        []*net.IPNet
	blacklist        []*net.IPNet
	whitelistLimiter *rate.Limiter // 全局共享的白名单限流器
	// skipPaths for static / admin auth that should not consume HTTP rate tokens
	// pull-session rate limit is handled separately in handlers
}

// rateLimiterEntry 限流器条目
type rateLimiterEntry struct {
	limiter    *rate.Limiter
	lastAccess time.Time
}

// InitGlobalLimiter 初始化全局限流器
func InitGlobalLimiter() *IPRateLimiter {
	cfg := config.GetConfig()

	whitelist := parseCIDRList(cfg.Security.WhiteList)
	blacklist := parseCIDRList(cfg.Security.BlackList)

	ratePerSecond := rate.Limit(float64(cfg.RateLimit.RequestLimit) / (cfg.RateLimit.PeriodHours * 3600))

	burstSize := cfg.RateLimit.RequestLimit

	limiter := &IPRateLimiter{
		ips:              make(map[string]*rateLimiterEntry),
		mu:               &sync.RWMutex{},
		r:                ratePerSecond,
		b:                burstSize,
		whitelist:        whitelist,
		blacklist:        blacklist,
		whitelistLimiter: rate.NewLimiter(rate.Inf, burstSize),
	}

	go limiter.cleanupRoutine()

	return limiter
}

// cleanupRoutine 定期清理过期的限流器
func (i *IPRateLimiter) cleanupRoutine() {
	ticker := time.NewTicker(CleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()
		expired := make([]string, 0)

		i.mu.RLock()
		for ip, entry := range i.ips {
			if now.Sub(entry.lastAccess) > 2*time.Hour {
				expired = append(expired, ip)
			}
		}
		i.mu.RUnlock()

		if len(expired) > 0 || len(i.ips) > MaxIPCacheSize {
			i.mu.Lock()
			for _, ip := range expired {
				delete(i.ips, ip)
			}

			if len(i.ips) > MaxIPCacheSize {
				i.ips = make(map[string]*rateLimiterEntry)
			}
			i.mu.Unlock()
		}
	}
}

// extractIPFromAddress 从地址中提取纯IP
func extractIPFromAddress(address string) string {
	if host, _, err := net.SplitHostPort(address); err == nil {
		return host
	}
	return address
}

// normalizeIPForRateLimit 标准化IP地址用于限流
func normalizeIPForRateLimit(ipStr string) string {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return ipStr
	}

	if ip.To4() != nil {
		return ipStr
	}

	ipv6 := ip.To16()
	for i := 8; i < 16; i++ {
		ipv6[i] = 0
	}
	return ipv6.String() + "/64"
}

// isIPInCIDRList 检查IP是否在CIDR列表中
func isIPInCIDRList(ip string, cidrList []*net.IPNet) bool {
	cleanIP := extractIPFromAddress(ip)
	parsedIP := net.ParseIP(cleanIP)
	if parsedIP == nil {
		return false
	}

	for _, cidr := range cidrList {
		if cidr.Contains(parsedIP) {
			return true
		}
	}
	return false
}

// GetLimiter 获取指定IP的限流器
func (i *IPRateLimiter) GetLimiter(ip string) (*rate.Limiter, bool) {
	cleanIP := extractIPFromAddress(ip)

	if isIPInCIDRList(cleanIP, i.blacklist) {
		return nil, false
	}

	if isIPInCIDRList(cleanIP, i.whitelist) {
		return i.whitelistLimiter, true
	}

	normalizedIP := normalizeIPForRateLimit(cleanIP)

	now := time.Now()

	var entry *rateLimiterEntry
	i.mu.RLock()
	_, exists := i.ips[normalizedIP]
	i.mu.RUnlock()

	if exists {
		i.mu.Lock()
		if entry, stillExists := i.ips[normalizedIP]; stillExists {
			entry.lastAccess = now
			i.mu.Unlock()
			return entry.limiter, true
		}
		i.mu.Unlock()
	}

	i.mu.Lock()
	if entry, exists := i.ips[normalizedIP]; exists {
		entry.lastAccess = now
		i.mu.Unlock()
		return entry.limiter, true
	}

	entry = &rateLimiterEntry{
		limiter:    rate.NewLimiter(i.r, i.b),
		lastAccess: now,
	}
	i.ips[normalizedIP] = entry
	i.mu.Unlock()

	return entry.limiter, true
}

// parseCIDRList parses IP/CIDR strings into IPNet list.
func parseCIDRList(items []string) []*net.IPNet {
	out := make([]*net.IPNet, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if !strings.Contains(item, "/") {
			if strings.Contains(item, ":") {
				item = item + "/128"
			} else {
				item = item + "/32"
			}
		}
		_, ipnet, err := net.ParseCIDR(item)
		if err == nil {
			out = append(out, ipnet)
		} else {
			fmt.Printf("警告: 无效的IP格式: %s\n", item)
		}
	}
	return out
}

// UpdateSecurity reloads whitelist/blacklist and rate parameters at runtime.
func (i *IPRateLimiter) UpdateSecurity(whiteList, blackList []string, requestLimit int, periodHours float64) {
	if requestLimit < 1 {
		requestLimit = 1
	}
	if periodHours <= 0 {
		periodHours = 1
	}
	ratePerSecond := rate.Limit(float64(requestLimit) / (periodHours * 3600))
	burstSize := requestLimit

	i.mu.Lock()
	defer i.mu.Unlock()
	i.whitelist = parseCIDRList(whiteList)
	i.blacklist = parseCIDRList(blackList)
	i.r = ratePerSecond
	i.b = burstSize
	i.whitelistLimiter = rate.NewLimiter(rate.Inf, burstSize)
	// reset per-IP limiters so new rate applies
	i.ips = make(map[string]*rateLimiterEntry)
}

// IsBlacklisted reports whether IP is on the blacklist.
func (i *IPRateLimiter) IsBlacklisted(ip string) bool {
	cleanIP := extractIPFromAddress(ip)
	i.mu.RLock()
	defer i.mu.RUnlock()
	return isIPInCIDRList(cleanIP, i.blacklist)
}

// IPInList checks whether ip matches any CIDR/IP entry in list.
func IPInList(ip string, list []string) bool {
	return isIPInCIDRList(extractIPFromAddress(ip), parseCIDRList(list))
}

// RateLimitMiddleware 速率限制中间件
func RateLimitMiddleware(limiter *IPRateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if path == "/" || path == "/images" || path == "/search" || path == "/admin" ||
			path == "/favicon.ico" ||
			strings.HasPrefix(path, "/assets/") ||
			strings.HasPrefix(path, "/admin/") {
			c.Next()
			return
		}

		// Admin API: only enforce blacklist, not token-bucket (login has own throttle)
		if strings.HasPrefix(path, "/api/admin") {
			cleanIP := extractIPFromAddress(c.ClientIP())
			if limiter.IsBlacklisted(cleanIP) {
				c.JSON(403, gin.H{"error": "您已被限制访问"})
				c.Abort()
				return
			}
			c.Next()
			return
		}

		// Docker registry blob/manifest requests: blacklist only at middleware;
		// pull-session quota is applied when a new pull session is created.
		isDockerRegistry := strings.HasPrefix(path, "/v2/") || strings.HasPrefix(path, "/token")
		cleanIP := extractIPFromAddress(c.ClientIP())

		ipLimiter, allowed := limiter.GetLimiter(cleanIP)

		if !allowed {
			c.JSON(403, gin.H{
				"error": "您已被限制访问",
			})
			c.Abort()
			return
		}

		if isDockerRegistry {
			// still protect against extreme abuse with a soft allow; actual pull quota is session-based
			c.Next()
			return
		}

		if !ipLimiter.Allow() {
			c.JSON(429, gin.H{
				"error": "请求频率过快，暂时限制访问",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
