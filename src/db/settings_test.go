package db

import (
	"encoding/json"
	"testing"
)

// display_name 的语义从「完整短语」改成了「服务商名」，前端按
// 「使用 {display_name} 登录 / 注册」拼接。存量的旧默认值必须迁移，
// 否则站点会显示成「使用 OAuth2 登录 注册」；但管理员自己填过的名字不能动。
func TestLoadOAuthMigratesLegacyDisplayNameOnly(t *testing.T) {
	useTestDB(t)
	if err := migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	if err := SetSetting(KeyOAuth, OAuthSettings{DisplayName: legacyOAuthDisplayName}); err != nil {
		t.Fatal(err)
	}
	if got := LoadOAuth().DisplayName; got == legacyOAuthDisplayName {
		t.Fatalf("旧默认值未被迁移，前端会拼出「使用 %s 注册」", legacyOAuthDisplayName)
	}

	if err := SetSetting(KeyOAuth, OAuthSettings{DisplayName: "GitHub"}); err != nil {
		t.Fatal(err)
	}
	if got := LoadOAuth().DisplayName; got != "GitHub" {
		t.Fatalf("自定义的 display_name 被改成了 %q", got)
	}
}

func TestDefaultFeatureTogglesDropRemovedAccelerationKeys(t *testing.T) {
	raw, err := json.Marshal(DefaultFeatureToggles())
	if err != nil {
		t.Fatalf("marshal defaults: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("unmarshal defaults: %v", err)
	}
	for _, key := range []string{"github", "huggingface"} {
		if _, ok := m[key]; ok {
			t.Fatalf("FeatureToggles still exposes %q: %s", key, raw)
		}
	}
	if m["docker_hub"] != true {
		t.Fatalf("docker_hub default lost: %s", raw)
	}
}

func TestLoadFeaturesToleratesLegacyGitHubKeys(t *testing.T) {
	useTestDB(t)
	if err := migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// image_search 刻意取 false：与 DefaultFeatureToggles 的 true 不同，
	// 这样若 LoadFeatures 走了"读取/反序列化失败即回退默认值"的兜底分支，
	// 断言就会失败，测试才真正区分得出解析路径与回退路径
	legacy := `{"docker_hub":true,"github":true,"huggingface":false,"image_search":false,` +
		`"offline_image":true,"public_mirror":false}`
	if _, err := DB.Exec(
		`INSERT INTO settings (key, value, updated_at) VALUES (?, ?, ?)`,
		KeyFeatures, legacy, Now(),
	); err != nil {
		t.Fatalf("insert legacy settings: %v", err)
	}

	got := LoadFeatures()
	if !got.DockerHub || !got.OfflineImage {
		t.Fatalf("legacy flags lost: %#v", got)
	}
	if got.ImageSearch {
		t.Fatalf("image_search should stay false, fell back to defaults: %#v", got)
	}
	if got.PublicMirror {
		t.Fatalf("public_mirror should stay false: %#v", got)
	}
}
