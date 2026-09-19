package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"hubproxy/db"
)

func TestBuildOAuthRedirectAddsWelcomeOnlyForNewAccounts(t *testing.T) {
	existing := buildOAuthRedirect("/admin", "tok123", false)
	if existing != "/admin#oauth_token=tok123" {
		t.Fatalf("existing-account redirect = %q", existing)
	}

	created := buildOAuthRedirect("/admin/change-password", "tok123", true)
	if created != "/admin/change-password#oauth_token=tok123&welcome=1" {
		t.Fatalf("new-account redirect = %q", created)
	}
}

func TestOAuthErrorRedirectsToLoginRoute(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	// gin.CreateTestContext 不会设置 Request，而 c.Redirect 内部的 http.Redirect 会读取它
	c.Request = httptest.NewRequest("GET", "/api/admin/oauth/callback", nil)

	redirectOAuthError(c, "state 无效或已过期")

	loc := w.Header().Get("Location")
	if !strings.HasPrefix(loc, "/login?oauth_error=") {
		t.Fatalf("Location = %q, want /login?oauth_error=...", loc)
	}
}

// oauthStartForTest 配置 OAuth 与管理员开关后调用 OAuthStart。
func oauthStartForTest(t *testing.T, loginEnabled, registerEnabled bool) *httptest.ResponseRecorder {
	t.Helper()
	seedRuntime(t)

	if err := db.SetSetting(db.KeyOAuth, db.OAuthSettings{
		Enabled:      true,
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		AuthURL:      "https://provider.example.com/authorize",
		TokenURL:     "https://provider.example.com/token",
		UserInfoURL:  "https://provider.example.com/userinfo",
		Scopes:       "openid profile",
		DisplayName:  "第三方账号",
	}); err != nil {
		t.Fatal(err)
	}

	admin := db.GlobalRuntime.GetAdmin()
	admin.OAuthLoginEnabled = loginEnabled
	admin.OAuthRegisterEnabled = registerEnabled
	if err := db.SetSetting(db.KeyAdmin, admin); err != nil {
		t.Fatal(err)
	}
	db.GlobalRuntime.Reload()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/admin/oauth/start?mode=login", nil)
	OAuthStart(c)
	return w
}

// 只开 OAuth 注册而关掉 OAuth 登录时，注册页上那个按钮点下去必须能走通。
// 准入条件若只看 OAuthLoginEnabled，这里会拿到 403，用户看到的是「OAuth 登录未开启」。
func TestOAuthStartAllowsRegisterOnly(t *testing.T) {
	w := oauthStartForTest(t, false, true)

	if w.Code != http.StatusFound {
		t.Fatalf("只开 OAuth 注册时应放行并跳转到服务商，实际 %d：%s", w.Code, w.Body.String())
	}
	if loc := w.Header().Get("Location"); !strings.Contains(loc, "provider.example.com") {
		t.Fatalf("Location = %q，应指向配置的授权地址", loc)
	}
}

// 登录与注册都关掉时必须仍然拒绝，放宽准入不等于取消准入。
func TestOAuthStartRejectsWhenBothDisabled(t *testing.T) {
	w := oauthStartForTest(t, false, false)

	if w.Code != http.StatusForbidden {
		t.Fatalf("两者都关闭时应 403，实际 %d：%s", w.Code, w.Body.String())
	}
}
