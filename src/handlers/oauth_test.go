package handlers

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
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
