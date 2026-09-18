package handlers

import "testing"

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
