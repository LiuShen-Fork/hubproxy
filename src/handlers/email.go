package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"hubproxy/db"
)

func AdminPutEmail(c *gin.Context) {
	var req db.EmailSettings
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数无效"})
		return
	}
	prev := db.GlobalRuntime.GetEmail()
	if req.Password == "" || req.Password == "********" {
		req.Password = prev.Password
	}
	if req.SMTPPort <= 0 {
		req.SMTPPort = 587
	}
	if err := db.SetSetting(db.KeyEmail, req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	db.GlobalRuntime.Reload()
	out := db.GlobalRuntime.GetEmail()
	if out.Password != "" {
		out.Password = "********"
	}
	c.JSON(http.StatusOK, gin.H{"email": out})
}

func AdminTestEmail(c *gin.Context) {
	var req struct {
		To string `json:"to"`
	}
	_ = c.ShouldBindJSON(&req)
	cfg := db.GlobalRuntime.GetEmail()
	to := strings.TrimSpace(req.To)
	if to == "" {
		to = cfg.From
	}
	if to == "" {
		to = cfg.Username
	}
	if to == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请填写测试收件邮箱"})
		return
	}
	site := db.GlobalRuntime.GetSite()
	subject := fmt.Sprintf("[%s] 邮件配置测试", site.Name)
	body := fmt.Sprintf("这是一封来自 %s 的测试邮件。\n时间：%s\n若收到此信，说明 SMTP 配置正常。\n",
		site.Name, time.Now().Format("2006-01-02 15:04:05"))
	if err := db.SendSMTPMail(cfg, to, subject, body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "测试邮件已发送至 " + to})
}

func AuthSendRegisterCode(c *gin.Context) {
	admin := db.GlobalRuntime.GetAdmin()
	emailCfg := db.GlobalRuntime.GetEmail()
	if !admin.FormRegisterAllowed() {
		c.JSON(http.StatusForbidden, gin.H{"error": "表单注册已关闭"})
		return
	}
	if !admin.EmailRegisterEnabled || !emailCfg.Enabled {
		c.JSON(http.StatusForbidden, gin.H{"error": "邮箱注册验证未开启"})
		return
	}
	var req struct {
		Email string `json:"email"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Email) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请填写邮箱"})
		return
	}
	code, err := db.CreateEmailCode(req.Email, "register", 15*time.Minute)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	site := db.GlobalRuntime.GetSite()
	subject := fmt.Sprintf("[%s] 注册验证码", site.Name)
	body := fmt.Sprintf("您的注册验证码是：%s\n15 分钟内有效。\n如非本人操作请忽略。\n", code)
	if err := db.SendSMTPMail(emailCfg, strings.TrimSpace(req.Email), subject, body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "验证码已发送"})
}
