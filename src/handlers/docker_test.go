package handlers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"hubproxy/config"
	"hubproxy/db"
)

// seedRuntime 复刻生产启动链路（main() 的 db.Init → db.SeedFromConfig），
// 这一步才会把 Registry 配置写进 GlobalRuntime。
//
// 为什么必须补上：resolveAuthHost / rewriteAuthHeader 读的是
// db.GlobalRuntime.GetRegistries()（SQLite 支撑的运行时快照），**不是** config.GetConfig()。
// config.toml 只在首次启动时作为种子写入 SQLite，之后以运行时快照为准。
// 所以只调 config.LoadConfig() 而不 seed，这两个函数看到的是一个空列表。
//
// db.Init 受 sync.Once 保护，只有本测试二进制内的首次调用生效，故用 :memory: 保持稳定，
// 不用 t.TempDir()（它会在首个测试结束时就删掉数据库文件）。
func seedRuntime(t *testing.T) {
	t.Helper()
	if err := db.Init(":memory:"); err != nil {
		t.Fatal(err)
	}
	if err := db.SeedFromConfig(config.GetConfig()); err != nil {
		t.Fatal(err)
	}
}

func TestParseRegistryPath(t *testing.T) {
	tests := []struct {
		path      string
		image     string
		apiType   string
		reference string
	}{
		{"library/nginx/manifests/latest", "library/nginx", "manifests", "latest"},
		{"library/nginx/blobs/sha256:abc", "library/nginx", "blobs", "sha256:abc"},
		{"library/nginx/tags/list", "library/nginx", "tags", "list"},
	}

	for _, tt := range tests {
		image, apiType, reference := parseRegistryPath(tt.path)
		if image != tt.image || apiType != tt.apiType || reference != tt.reference {
			t.Fatalf("parseRegistryPath(%q) = %q %q %q", tt.path, image, apiType, reference)
		}
	}
}

func TestParseRegistryPathInvalid(t *testing.T) {
	image, apiType, reference := parseRegistryPath("library/nginx/unknown/latest")
	if image != "" || apiType != "" || reference != "" {
		t.Fatalf("invalid path parsed as %q %q %q", image, apiType, reference)
	}
}

func TestResolveAuthHost(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	data := []byte(`
[registries."ghcr.io"]
upstream = "ghcr.io"
authHost = "ghcr.io/token"
authType = "github"
enabled = true

[registries."quay.io"]
upstream = "quay.io"
authHost = "quay.io/v2/auth"
authType = "quay"
enabled = true

[registries."registry.example.com"]
upstream = "registry.example.com"
authHost = "auth.example.com/token"
authType = "oauth2"
enabled = true
`)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CONFIG_PATH", path)
	if err := config.LoadConfig(); err != nil {
		t.Fatal(err)
	}
	seedRuntime(t)

	if got := resolveAuthHost(""); got != "" {
		t.Fatalf("empty service = %q", got)
	}
	if got := resolveAuthHost("registry.docker.io"); got != "" {
		t.Fatalf("docker hub service = %q", got)
	}
	if got := resolveAuthHost("ghcr.io"); got != "ghcr.io/token" {
		t.Fatalf("ghcr.io = %q", got)
	}
	if got := resolveAuthHost("quay.io"); got != "quay.io/v2/auth" {
		t.Fatalf("quay.io = %q", got)
	}
	// registry.example.com 不在内置默认表 DefaultRegistryToggles() 里，只存在于上面的
	// config.toml。前三条断言用的 ghcr.io / quay.io 在默认表里恰好有相同的 AuthHost，
	// 所以它们即使配置被完全忽略也会通过；这条才是真正证明「配置驱动映射」的断言。
	if got := resolveAuthHost("registry.example.com"); got != "auth.example.com/token" {
		t.Fatalf("registry.example.com = %q", got)
	}
}

func TestBuildDockerAuthURL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	data := []byte(`
[registries."ghcr.io"]
upstream = "ghcr.io"
authHost = "ghcr.io/token"
enabled = true
`)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CONFIG_PATH", path)
	if err := config.LoadConfig(); err != nil {
		t.Fatal(err)
	}
	seedRuntime(t)

	gin.SetMode(gin.TestMode)

	t.Run("docker hub keeps path", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodGet, "/token?service=registry.docker.io&scope=repository:library/nginx:pull", nil)
		got := buildDockerAuthURL(c)
		want := "https://auth.docker.io/token?service=registry.docker.io&scope=repository:library/nginx:pull"
		if got != want {
			t.Fatalf("got %q want %q", got, want)
		}
	})

	t.Run("ghcr uses AuthHost without duplicating path", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodGet, "/token?service=ghcr.io&scope=repository:foo/bar:pull", nil)
		got := buildDockerAuthURL(c)
		want := "https://ghcr.io/token?service=ghcr.io&scope=repository:foo/bar:pull"
		if got != want {
			t.Fatalf("got %q want %q", got, want)
		}
	})
}

func TestRewriteAuthHeader(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	data := []byte(`
[registries."quay.io"]
upstream = "quay.io"
authHost = "quay.io/v2/auth"
enabled = true

[registries."ghcr.io"]
upstream = "ghcr.io"
authHost = "ghcr.io/token"
enabled = true
`)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CONFIG_PATH", path)
	if err := config.LoadConfig(); err != nil {
		t.Fatal(err)
	}
	seedRuntime(t)

	got := rewriteAuthHeader(`Bearer realm="https://quay.io/v2/auth",service="quay.io"`, "proxy.example.com")
	want := `Bearer realm="http://proxy.example.com/token",service="quay.io"`
	if got != want {
		t.Fatalf("quay rewrite: got %q want %q", got, want)
	}

	got = rewriteAuthHeader(`Bearer realm="https://ghcr.io/token",service="ghcr.io"`, "proxy.example.com")
	want = `Bearer realm="http://proxy.example.com/token",service="ghcr.io"`
	if got != want {
		t.Fatalf("ghcr rewrite: got %q want %q", got, want)
	}

	got = rewriteAuthHeader(`Bearer realm="https://auth.docker.io/token",service="registry.docker.io"`, "proxy.example.com")
	want = `Bearer realm="http://proxy.example.com/token",service="registry.docker.io"`
	if got != want {
		t.Fatalf("docker hub rewrite: got %q want %q", got, want)
	}
}
