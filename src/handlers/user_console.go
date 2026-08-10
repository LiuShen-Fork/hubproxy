package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"hubproxy/db"
)

func UserDashboard(c *gin.Context) {
	u := currentUser(c)
	days, _ := strconv.Atoi(c.DefaultQuery("days", "14"))
	stats, err := db.GetUserDashboardStats(u.ID, days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "code": "INTERNAL_ERROR"})
		return
	}
	quota, _ := db.GetUserQuota(u.ID)
	c.JSON(http.StatusOK, gin.H{
		"stats": stats,
		"quota": quota,
	})
}

func UserQuota(c *gin.Context) {
	u := currentUser(c)
	quota, err := db.GetUserQuota(u.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "code": "INTERNAL_ERROR"})
		return
	}
	c.JSON(http.StatusOK, quota)
}

func UserListPulls(c *gin.Context) {
	u := currentUser(c)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	list, total, err := db.ListPullSessions(db.PullListFilter{
		IP:       c.Query("ip"),
		Image:    c.Query("image"),
		Category: c.Query("category"),
		Registry: c.Query("registry"),
		Status:   c.Query("status"),
		UserID:   u.ID,
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": list, "total": total, "page": page, "page_size": pageSize})
}

func UserGetToken(c *gin.Context) {
	u := currentUser(c)
	tok, err := db.EnsureUserAccessToken(u.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	host := c.Request.Host
	feat := db.GlobalRuntime.GetFeatures()
	// require_token = 必须使用令牌路径 = 未开启公共镜像
	requireToken := !feat.AllowPublicDockerPull()
	c.JSON(http.StatusOK, gin.H{
		"token":          tok,
		"pull_path":      tok.Token,
		"examples":       buildTokenExamples(host, tok.Token),
		"require_token":  requireToken,
		"public_mirror":  feat.PublicMirror,
	})
}

func UserResetToken(c *gin.Context) {
	u := currentUser(c)
	tok, err := db.ResetUserAccessToken(u.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	host := c.Request.Host
	c.JSON(http.StatusOK, gin.H{
		"token":    tok,
		"examples": buildTokenExamples(host, tok.Token),
		"message":  "令牌已重置，旧令牌立即失效且不可复用",
	})
}

func UserGetIPWhitelist(c *gin.Context) {
	u := currentUser(c)
	list, err := db.ListUserIPWhitelist(u.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": list})
}

func UserPutIPWhitelist(c *gin.Context) {
	u := currentUser(c)
	var req struct {
		Items []string `json:"items"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数无效"})
		return
	}
	if req.Items == nil {
		req.Items = []string{}
	}
	if err := db.SetUserIPWhitelist(u.ID, req.Items); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	list, _ := db.ListUserIPWhitelist(u.ID)
	c.JSON(http.StatusOK, gin.H{"items": list})
}

func UserAddIPWhitelist(c *gin.Context) {
	u := currentUser(c)
	var req struct {
		IP string `json:"ip" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数无效"})
		return
	}
	if err := db.AddUserIPWhitelist(u.ID, req.IP); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	list, _ := db.ListUserIPWhitelist(u.ID)
	c.JSON(http.StatusOK, gin.H{"items": list})
}

func UserRemoveIPWhitelist(c *gin.Context) {
	u := currentUser(c)
	ip := strings.TrimSpace(c.Query("ip"))
	if ip == "" {
		var req struct {
			IP string `json:"ip"`
		}
		_ = c.ShouldBindJSON(&req)
		ip = strings.TrimSpace(req.IP)
	}
	if ip == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 IP"})
		return
	}
	_ = db.RemoveUserIPWhitelist(u.ID, ip)
	list, _ := db.ListUserIPWhitelist(u.ID)
	c.JSON(http.StatusOK, gin.H{"items": list})
}

func UserGuide(c *gin.Context) {
	u := currentUser(c)
	tok, _ := db.EnsureUserAccessToken(u.ID)
	host := c.Request.Host
	token := ""
	if tok != nil {
		token = tok.Token
	}
	feat := db.GlobalRuntime.GetFeatures()
	c.JSON(http.StatusOK, gin.H{
		"host":          host,
		"token":         token,
		"require_token": !feat.AllowPublicDockerPull(),
		"public_mirror": feat.PublicMirror,
		"examples":      buildTokenExamples(host, token),
		"notes": []string{
			"方式 A（推荐显式）：docker pull " + host + "/你的令牌/镜像名:标签",
			"方式 B（daemon.json）：registry-mirrors 配置为 https://" + host + "/你的令牌 ，之后可直接 docker pull nginx",
			"第三方源：docker pull " + host + "/令牌/ghcr.io/owner/app:tag",
			"请勿在浏览器中直接打开 /令牌 路径（会返回 404，避免被搜索引擎收录）",
			"重置令牌后旧令牌立即失效，且永不复用；若用了 daemon.json 需同步改 mirror 路径",
			"用户 IP 白名单为空表示不限制 IP；配置后仅允许列表内 IP 使用你的令牌",
			"每日拉取次数按本地时间 0 点重置，可在用户概览查看剩余次数",
		},
	})
}

func buildTokenExamples(host, token string) map[string]string {
	if token == "" {
		token = "YOURTOKEN"
	}
	return map[string]string{
		"hub_official": "docker pull " + host + "/" + token + "/nginx:latest",
		"hub_user":     "docker pull " + host + "/" + token + "/library/nginx:latest",
		"ghcr":         "docker pull " + host + "/" + token + "/ghcr.io/owner/app:latest",
		"k8s":          "docker pull " + host + "/" + token + "/registry.k8s.io/pause:3.9",
		"gitlab":       "docker pull " + host + "/" + token + "/registry.gitlab.com/group/project:tag",
		"daemon_json":  "{\n  \"registry-mirrors\": [\"https://" + host + "/" + token + "\"]\n}",
		"daemon_pull":  "docker pull nginx:latest   # 配合上面的 registry-mirrors",
	}
}
