package db

import (
	"encoding/json"
	"testing"
)

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

	legacy := `{"docker_hub":true,"github":true,"huggingface":false,"image_search":true,` +
		`"offline_image":true,"public_mirror":false}`
	if _, err := DB.Exec(
		`INSERT INTO settings (key, value, updated_at) VALUES (?, ?, ?)`,
		KeyFeatures, legacy, Now(),
	); err != nil {
		t.Fatalf("insert legacy settings: %v", err)
	}

	got := LoadFeatures()
	if !got.DockerHub || !got.ImageSearch || !got.OfflineImage {
		t.Fatalf("legacy flags lost: %#v", got)
	}
	if got.PublicMirror {
		t.Fatalf("public_mirror should stay false: %#v", got)
	}
}
