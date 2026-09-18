package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"hubproxy/config"
	"hubproxy/db"
	"hubproxy/handlers"
	"hubproxy/utils"
)

func newTestRouter(t *testing.T, configBody string) *gin.Engine {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(configBody), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("CONFIG_PATH", path)
	if err := config.LoadConfig(); err != nil {
		t.Fatal(err)
	}

	// 复刻生产启动链路（main() 的 db.Init → db.SeedFromConfig），这一步才会填充
	// GlobalRuntime。跳过它 GlobalRuntime 就是零值结构体：所有功能开关为 false
	// （离线下载、公共镜像闸门会把请求 403 掉）、Registry 列表为空。
	//
	// 用 :memory: 而不是 t.TempDir()：db.Init 受 sync.Once 保护只有首次调用生效，
	// 而 t.TempDir() 会在首个测试结束时就删掉，后续测试会指向一个已消失的文件。
	if err := db.Init(":memory:"); err != nil {
		t.Fatal(err)
	}
	if err := db.SeedFromConfig(config.GetConfig()); err != nil {
		t.Fatal(err)
	}

	utils.InitHTTPClients()
	globalLimiter = utils.InitGlobalLimiter()
	handlers.InitDockerProxy()
	handlers.InitImageStreamer()
	handlers.InitDebouncer()

	return buildRouter(config.GetConfig())
}

// setPublicMirror 切换公共镜像开关。功能闸门是每请求求值的（不是构建路由时快照的），
// 所以在 newTestRouter 之后调用同样生效。
func setPublicMirror(t *testing.T, enabled bool) {
	t.Helper()
	features := db.DefaultFeatureToggles()
	features.PublicMirror = enabled
	if err := db.SetSetting(db.KeyFeatures, features); err != nil {
		t.Fatal(err)
	}
	db.GlobalRuntime.Reload()
}

func performRequest(router http.Handler, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("User-Agent", "hubproxy-test")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func TestReadyRoute(t *testing.T) {
	router := newTestRouter(t, "")

	w := performRequest(router, http.MethodGet, "/ready", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}

	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	// service 是健康检查的信息性字段，fork 后品牌更名为「清羽镜像」。
	if got["ready"] != true || got["service"] != "qingyu-mirror" {
		t.Fatalf("unexpected ready response: %#v", got)
	}
}

func TestFrontendDisabledRoutesReturnNotFound(t *testing.T) {
	router := newTestRouter(t, `
[server]
enableFrontend = false
`)

	for _, path := range []string{"/", "/images", "/search", "/favicon.ico"} {
		w := performRequest(router, http.MethodGet, path, "")
		if w.Code != http.StatusNotFound {
			t.Fatalf("%s status = %d, want 404", path, w.Code)
		}
	}
}

func TestSingleImageDownloadPrepareReturnsURL(t *testing.T) {
	router := newTestRouter(t, "")

	w := performRequest(router, http.MethodGet, "/api/image/download?image=nginx&mode=prepare", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}

	var got struct {
		DownloadURL string `json:"download_url"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got.DownloadURL, "image=nginx") || !strings.Contains(got.DownloadURL, "token=") {
		t.Fatalf("download_url = %q", got.DownloadURL)
	}
	if !strings.HasPrefix(got.DownloadURL, "/api/image/download?") {
		t.Fatalf("download_url = %q", got.DownloadURL)
	}
}

func TestBatchImageDownloadPrepareReturnsURL(t *testing.T) {
	router := newTestRouter(t, "")

	body := `{"images":["nginx"],"useCompressedLayers":true}`
	w := performRequest(router, http.MethodPost, "/api/image/batch?mode=prepare", body)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}

	var got struct {
		DownloadURL string `json:"download_url"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got.DownloadURL, "/api/image/batch?token=") {
		t.Fatalf("download_url = %q", got.DownloadURL)
	}
}

func TestBatchImageDownloadRejectsTooManyImages(t *testing.T) {
	router := newTestRouter(t, `
[download]
maxImages = 1
`)

	body := `{"images":["nginx","redis"],"useCompressedLayers":true}`
	w := performRequest(router, http.MethodPost, "/api/image/batch?mode=prepare", body)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", w.Code, w.Body.String())
	}
}

func TestDockerV2PingAndInvalidPath(t *testing.T) {
	router := newTestRouter(t, "")

	// 本用例验的是路径解析与校验（400）。默认 public_mirror=false 时，匿名请求会先被
	// 「公共镜像已关闭」闸门拦成 403，永远走不到路径校验那一步。这里显式打开闸门，
	// 让断言真正覆盖它要覆盖的逻辑；产品的默认值不受影响。
	setPublicMirror(t, true)

	w := performRequest(router, http.MethodGet, "/v2/", "")
	if w.Code != http.StatusOK {
		t.Fatalf("/v2/ status = %d, want 200; body=%s", w.Code, w.Body.String())
	}

	w = performRequest(router, http.MethodGet, "/v2/library/nginx/unknown/latest", "")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("invalid v2 status = %d, want 400; body=%s", w.Code, w.Body.String())
	}
}

func TestSearchAPIRejectsMissingQuery(t *testing.T) {
	router := newTestRouter(t, `
[server]
enableFrontend = false
`)

	w := performRequest(router, http.MethodGet, "/api/search", "")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", w.Code, w.Body.String())
	}

	var got map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["error"] == "" {
		t.Fatalf("missing error response: %#v", got)
	}
}

func TestSearchServesSPAWhenFrontendEnabled(t *testing.T) {
	router := newTestRouter(t, `
[server]
enableFrontend = true
`)

	w := performRequest(router, http.MethodGet, "/search?q=nginx", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("content-type = %q, want text/html", w.Header().Get("Content-Type"))
	}
	if !strings.Contains(w.Body.String(), `<div id="app">`) {
		t.Fatalf("SPA shell missing: %s", w.Body.String())
	}
}

func TestRemovedAccelerationPathsReturnNotFound(t *testing.T) {
	router := newTestRouter(t, "")

	for _, path := range []string{
		"/gh/owner/repo/raw/main/a.txt",
		"/hf/owner/model/resolve/main/x.bin",
		"/https://example.com/file.zip",
	} {
		w := performRequest(router, http.MethodGet, path, "")
		if w.Code != http.StatusNotFound {
			t.Fatalf("%s status = %d, want 404; body=%s", path, w.Code, w.Body.String())
		}
	}
}

func TestUnmatchedPathReturnsJSONNotFound(t *testing.T) {
	router := newTestRouter(t, "")

	w := performRequest(router, http.MethodGet, "/nonexistent", "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body=%s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Fatalf("content-type = %q, want application/json", ct)
	}
	var got map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("body is not JSON: %v; body=%s", err, w.Body.String())
	}
	if got["error"] != "页面不存在" || got["code"] != "NOT_FOUND" {
		t.Fatalf("unexpected body: %#v", got)
	}
}

func TestUnmatchedAPIPathReturnsJSONNotFound(t *testing.T) {
	router := newTestRouter(t, "")

	w := performRequest(router, http.MethodGet, "/api/nonexistent", "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body=%s", w.Code, w.Body.String())
	}
	var got map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("body is not JSON: %v; body=%s", err, w.Body.String())
	}
	if got["code"] != "NOT_FOUND" {
		t.Fatalf("unexpected body: %#v", got)
	}
}

func TestTokenBrowsePathKeepsNoindex(t *testing.T) {
	router := newTestRouter(t, "")

	w := performRequest(router, http.MethodGet, "/Ab12Cd34", "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body=%s", w.Code, w.Body.String())
	}
	if tag := w.Header().Get("X-Robots-Tag"); !strings.Contains(tag, "noindex") {
		t.Fatalf("X-Robots-Tag = %q, want noindex", tag)
	}
	// 按设计文档 3.2，该路径必须同时带 Cache-Control: no-store
	if cc := w.Header().Get("Cache-Control"); !strings.Contains(cc, "no-store") {
		t.Fatalf("Cache-Control = %q, want no-store", cc)
	}
}

func TestAuthPagesServeSPAWhenFrontendEnabled(t *testing.T) {
	router := newTestRouter(t, `
[server]
enableFrontend = true
`)

	for _, path := range []string{"/login", "/register"} {
		w := performRequest(router, http.MethodGet, path, "")
		if w.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want 200; body=%s", path, w.Code, w.Body.String())
		}
		if !strings.Contains(w.Header().Get("Content-Type"), "text/html") {
			t.Fatalf("%s content-type = %q, want text/html", path, w.Header().Get("Content-Type"))
		}
	}
}

// 说明：本用例只是"前瞻性护栏"。前端关闭时 /admin/login 由既有的
// GET /admin/*path → notFound 通配兜住，在本分支之前的基线提交上同样是 404，
// 因此它并不能证明本分支删掉了这条路由；保留它是为了防止日后有人把
// /admin/login 重新注册回来。
func TestOldAdminLoginRouteIsGone(t *testing.T) {
	router := newTestRouter(t, `
[server]
enableFrontend = false
`)

	w := performRequest(router, http.MethodGet, "/admin/login", "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("/admin/login status = %d, want 404; body=%s", w.Code, w.Body.String())
	}
}
