package db

import "testing"

func TestCreateOAuthUserRequiresPasswordChange(t *testing.T) {
	useTestDB(t)
	if err := migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	user, err := CreateOAuthUser("octocat", "octo@example.com")
	if err != nil {
		t.Fatalf("create oauth user: %v", err)
	}
	if !user.MustChangePassword {
		t.Fatal("oauth-created user must be forced to set a usable password")
	}
}
