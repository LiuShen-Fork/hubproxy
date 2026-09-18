package utils

import (
	"os"
	"path/filepath"
	"testing"

	"hubproxy/config"
)

func TestParseDockerImage(t *testing.T) {
	tests := []struct {
		name       string
		image      string
		namespace  string
		repository string
		tag        string
		fullName   string
	}{
		{"official", "nginx", "library", "nginx", "latest", "library/nginx"},
		{"tagged", "redis:7", "library", "redis", "7", "library/redis"},
		{"namespaced", "user/app:v1", "user", "app", "v1", "user/app"},
		{"registry", "ghcr.io/user/app:v2", "user", "app", "v2", "user/app"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GlobalAccessController.ParseDockerImage(tt.image)
			if got.Namespace != tt.namespace || got.Repository != tt.repository || got.Tag != tt.tag || got.FullName != tt.fullName {
				t.Fatalf("ParseDockerImage(%q) = %#v", tt.image, got)
			}
		})
	}
}

func TestDockerAccessLists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	data := []byte(`
[access]
whiteList = ["library/*", "good/*"]
blackList = ["good/bad"]
`)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CONFIG_PATH", path)
	if err := config.LoadConfig(); err != nil {
		t.Fatal(err)
	}

	if allowed, reason := GlobalAccessController.CheckDockerAccess("nginx"); !allowed {
		t.Fatalf("nginx denied: %s", reason)
	}
	if allowed, _ := GlobalAccessController.CheckDockerAccess("good/bad:latest"); allowed {
		t.Fatal("blacklisted image allowed")
	}
	if allowed, _ := GlobalAccessController.CheckDockerAccess("other/app"); allowed {
		t.Fatal("image outside whitelist allowed")
	}
}
