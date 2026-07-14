package handlers

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"hubproxy/db"
	"hubproxy/utils"
)

// countingWriter wraps ResponseWriter to count written bytes.
type countingWriter struct {
	gin.ResponseWriter
	n int64
}

func (w *countingWriter) Write(b []byte) (int, error) {
	n, err := w.ResponseWriter.Write(b)
	w.n += int64(n)
	return n, err
}

func (w *countingWriter) WriteString(s string) (int, error) {
	n, err := w.ResponseWriter.WriteString(s)
	w.n += int64(n)
	return n, err
}

// trackDockerPull groups layer/manifest requests into one pull session.
// Returns (session, deniedReason). If deniedReason != "", caller should reject.
func trackDockerPull(c *gin.Context, imageName, registry, tag, eventType, reference string) (*db.PullSession, string) {
	if imageName == "" {
		return nil, ""
	}
	if registry == "" {
		registry = "docker.io"
	}
	if tag == "" {
		if strings.HasPrefix(reference, "sha256:") {
			tag = "digest"
		} else if reference != "" {
			tag = reference
		} else {
			tag = "latest"
		}
	}
	ip := c.ClientIP()
	userID, accessToken := accessUserFromContext(c)

	existing, err := db.FindActivePullSession(ip, imageName, registry, userID)
	if err != nil {
		fmt.Printf("pull session lookup error: %v\n", err)
	}
	if existing == nil {
		if ok, reason := CheckPullQuota(ip, userID); !ok {
			return nil, reason
		}
	}

	sess, _, err := db.FindOrCreatePullSession(ip, imageName, registry, tag, userID, accessToken)
	if err != nil {
		fmt.Printf("pull session error: %v\n", err)
		return nil, ""
	}
	c.Set("pull_session_id", sess.ID)
	return sess, ""
}

func recordPullBytes(c *gin.Context, eventType, reference string, bytes int64, statusCode int) {
	id, ok := c.Get("pull_session_id")
	if !ok {
		return
	}
	sessionID, _ := id.(string)
	if sessionID == "" {
		return
	}
	if err := db.RecordPullEvent(sessionID, eventType, reference, bytes, statusCode); err != nil {
		fmt.Printf("record pull event error: %v\n", err)
	}
}

// CheckPullQuota returns false if IP/user exceeded pull session limit.
// User-scoped pulls use per-user daily limit (local midnight Asia/Shanghai).
func CheckPullQuota(ip string, userID int64) (bool, string) {
	// whitelist IPs skip global IP quota only
	if userID > 0 {
		q, err := db.GetUserPullQuota(userID)
		if err != nil {
			return true, ""
		}
		if !q.Unlimited && q.Remaining <= 0 {
			return false, fmt.Sprintf("今日拉取次数已用尽（%d/%d，每日 0 点刷新）", q.UsedToday, q.DailyLimit)
		}
		return true, ""
	}

	if globalLimiterExempt(ip) {
		return true, ""
	}
	rl := db.GlobalRuntime.GetRateLimit()
	if rl.PullLimit <= 0 {
		return true, ""
	}
	n, err := db.CountPullsByIPInPeriod(ip, rl.PeriodHours)
	if err != nil {
		return true, ""
	}
	if n >= rl.PullLimit {
		return false, fmt.Sprintf("拉取次数超限（%d/%g小时）", rl.PullLimit, rl.PeriodHours)
	}
	return true, ""
}

// optional hook: whitelist exemption via security settings CIDR
func globalLimiterExempt(ip string) bool {
	sec := db.GlobalRuntime.GetSecurity()
	return utils.IPInList(ip, sec.WhiteList)
}
