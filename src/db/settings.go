package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"sync"
)

const (
	KeyRateLimit   = "rate_limit"
	KeySecurity    = "security"
	KeyAccess      = "access"
	KeyAdmin       = "admin"
	KeyPullSession = "pull_session"
	KeyFeatures    = "features"
	KeyRegistries  = "registries"
)

type RateLimitSettings struct {
	RequestLimit int     `json:"request_limit"`
	PeriodHours  float64 `json:"period_hours"`
	// PullLimit: max complete image pulls per IP per period (session-based)
	PullLimit int `json:"pull_limit"`
}

type SecuritySettings struct {
	WhiteList []string `json:"white_list"`
	BlackList []string `json:"black_list"`
}

type AccessSettings struct {
	WhiteList []string `json:"white_list"`
	BlackList []string `json:"black_list"`
	Proxy     string   `json:"proxy"`
}

type AdminSettings struct {
	RegisterEnabled bool `json:"register_enabled"`
}

type PullSessionSettings struct {
	// WindowMinutes: how long to keep a session "active" for matching same pull's layers
	WindowMinutes int `json:"window_minutes"`
	// IdleMinutes: mark active session completed after idle
	IdleMinutes int `json:"idle_minutes"`
	// RePullGapSeconds: if a session already downloaded layers and is idle this long,
	// a new tag-manifest starts a new countable pull (second docker pull).
	RePullGapSeconds int `json:"re_pull_gap_seconds"`
}

// FeatureToggles controls each acceleration path.
type FeatureToggles struct {
	DockerHub    bool `json:"docker_hub"`
	GitHub       bool `json:"github"`
	HuggingFace  bool `json:"huggingface"`
	ImageSearch  bool `json:"image_search"`
	OfflineImage bool `json:"offline_image"`
	// PublicMirror: when true, allow docker pull without user token path
	PublicMirror bool `json:"public_mirror"`
	// RequireUserToken kept for backward-compatible JSON; ignored if PublicMirror is set in new clients
	RequireUserToken bool `json:"require_user_token,omitempty"`
}

type RegistryToggle struct {
	Domain   string `json:"domain"`
	Upstream string `json:"upstream"`
	AuthHost string `json:"auth_host"`
	AuthType string `json:"auth_type"`
	Enabled  bool   `json:"enabled"`
	Label    string `json:"label"`
}

var settingsMu sync.RWMutex

func GetSetting[T any](key string, dest *T) error {
	var raw string
	err := DB.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&raw)
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(raw), dest)
}

func SetSetting(key string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	settingsMu.Lock()
	defer settingsMu.Unlock()
	_, err = DB.Exec(
		`INSERT INTO settings (key, value, updated_at) VALUES (?, ?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		key, string(raw), Now(),
	)
	return err
}

func DefaultFeatureToggles() FeatureToggles {
	return FeatureToggles{
		DockerHub:    true,
		GitHub:       true,
		HuggingFace:  true,
		ImageSearch:  true,
		OfflineImage: true,
		PublicMirror: false, // 默认关闭公共镜像，需令牌
	}
}

func DefaultRegistryToggles() []RegistryToggle {
	return []RegistryToggle{
		{Domain: "ghcr.io", Upstream: "ghcr.io", AuthHost: "ghcr.io/token", AuthType: "github", Enabled: true, Label: "GitHub Container Registry"},
		{Domain: "gcr.io", Upstream: "gcr.io", AuthHost: "gcr.io/v2/token", AuthType: "google", Enabled: true, Label: "Google Container Registry"},
		{Domain: "quay.io", Upstream: "quay.io", AuthHost: "quay.io/v2/auth", AuthType: "quay", Enabled: true, Label: "Quay.io"},
		{Domain: "registry.k8s.io", Upstream: "registry.k8s.io", AuthHost: "registry.k8s.io", AuthType: "anonymous", Enabled: true, Label: "Kubernetes"},
		{Domain: "registry.gitlab.com", Upstream: "registry.gitlab.com", AuthHost: "gitlab.com/jwt/auth", AuthType: "gitlab", Enabled: true, Label: "GitLab Registry"},
	}
}

// removedRegistryDomains are pruned from stored settings on upgrade.
var removedRegistryDomains = map[string]bool{
	"nvcr.io":            true,
	"k8s.gcr.io":         true,
	"mcr.microsoft.com":  true,
	"public.ecr.aws":     true,
	"docker.elastic.co":  true,
}

func EnsureDefaultSettings(rateLimit RateLimitSettings, security SecuritySettings, access AccessSettings) error {
	defaults := map[string]any{
		KeyRateLimit: rateLimit,
		KeySecurity:  security,
		KeyAccess:    access,
		KeyAdmin: AdminSettings{
			RegisterEnabled: false,
		},
		KeyPullSession: PullSessionSettings{
			WindowMinutes:    15,
			IdleMinutes:      30,
			RePullGapSeconds: 120,
		},
		KeyFeatures:   DefaultFeatureToggles(),
		KeyRegistries: DefaultRegistryToggles(),
	}
	for k, v := range defaults {
		var existing string
		err := DB.QueryRow(`SELECT value FROM settings WHERE key = ?`, k).Scan(&existing)
		if err == nil && existing != "" {
			// merge new registry domains into existing list
			if k == KeyRegistries {
				_ = mergeRegistryDefaults()
			}
			continue
		}
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if err := SetSetting(k, v); err != nil {
			return err
		}
	}
	return nil
}

func mergeRegistryDefaults() error {
	existing := LoadRegistries()
	// prune removed sources (e.g. nvcr.io)
	filtered := make([]RegistryToggle, 0, len(existing))
	changed := false
	for _, r := range existing {
		if removedRegistryDomains[r.Domain] {
			changed = true
			continue
		}
		filtered = append(filtered, r)
	}
	existing = filtered
	byDomain := map[string]RegistryToggle{}
	for _, r := range existing {
		byDomain[r.Domain] = r
	}
	for _, d := range DefaultRegistryToggles() {
		if _, ok := byDomain[d.Domain]; !ok {
			existing = append(existing, d)
			changed = true
		}
	}
	if !changed {
		return nil
	}
	return SetSetting(KeyRegistries, existing)
}

func LoadRateLimit() RateLimitSettings {
	var s RateLimitSettings
	if err := GetSetting(KeyRateLimit, &s); err != nil || s.RequestLimit <= 0 {
		return RateLimitSettings{RequestLimit: 500, PeriodHours: 3, PullLimit: 50}
	}
	if s.PullLimit <= 0 {
		s.PullLimit = 50
	}
	return s
}

func LoadSecurity() SecuritySettings {
	var s SecuritySettings
	if err := GetSetting(KeySecurity, &s); err != nil {
		return SecuritySettings{WhiteList: []string{}, BlackList: []string{}}
	}
	if s.WhiteList == nil {
		s.WhiteList = []string{}
	}
	if s.BlackList == nil {
		s.BlackList = []string{}
	}
	return s
}

func LoadAccess() AccessSettings {
	var s AccessSettings
	if err := GetSetting(KeyAccess, &s); err != nil {
		return AccessSettings{WhiteList: []string{}, BlackList: []string{}}
	}
	if s.WhiteList == nil {
		s.WhiteList = []string{}
	}
	if s.BlackList == nil {
		s.BlackList = []string{}
	}
	return s
}

func LoadAdmin() AdminSettings {
	var s AdminSettings
	if err := GetSetting(KeyAdmin, &s); err != nil {
		return AdminSettings{RegisterEnabled: false}
	}
	return s
}

func LoadPullSession() PullSessionSettings {
	var s PullSessionSettings
	if err := GetSetting(KeyPullSession, &s); err != nil || s.WindowMinutes <= 0 {
		return PullSessionSettings{WindowMinutes: 15, IdleMinutes: 30, RePullGapSeconds: 120}
	}
	if s.IdleMinutes <= 0 {
		s.IdleMinutes = 30
	}
	if s.RePullGapSeconds <= 0 {
		s.RePullGapSeconds = 120
	}
	return s
}

func LoadFeatures() FeatureToggles {
	def := DefaultFeatureToggles()
	var s FeatureToggles
	if err := GetSetting(KeyFeatures, &s); err != nil {
		return def
	}
	// migrate old require_user_token → public_mirror
	// if old installs only had require_user_token=true, public_mirror should stay false
	if s.RequireUserToken && !s.PublicMirror {
		s.PublicMirror = false
	}
	// if JSON had require_user_token:false explicitly and public_mirror zero → public on
	// Detect via raw map
	var raw map[string]any
	if GetSetting(KeyFeatures, &raw) == nil {
		if _, hasPublic := raw["public_mirror"]; !hasPublic {
			if v, ok := raw["require_user_token"].(bool); ok {
				s.PublicMirror = !v
			}
		}
	}
	return s
}

// AllowPublicDockerPull is true when anonymous/no-token docker pulls are allowed.
func (f FeatureToggles) AllowPublicDockerPull() bool {
	return f.PublicMirror
}

func LoadRegistries() []RegistryToggle {
	var list []RegistryToggle
	if err := GetSetting(KeyRegistries, &list); err != nil || len(list) == 0 {
		return DefaultRegistryToggles()
	}
	out := make([]RegistryToggle, 0, len(list))
	for _, r := range list {
		if removedRegistryDomains[r.Domain] {
			continue
		}
		out = append(out, r)
	}
	if len(out) == 0 {
		return DefaultRegistryToggles()
	}
	return out
}

func RegistryMapFromSettings() map[string]struct {
	Upstream string
	AuthHost string
	AuthType string
	Enabled  bool
} {
	out := map[string]struct {
		Upstream string
		AuthHost string
		AuthType string
		Enabled  bool
	}{}
	for _, r := range LoadRegistries() {
		out[r.Domain] = struct {
			Upstream string
			AuthHost string
			AuthType string
			Enabled  bool
		}{Upstream: r.Upstream, AuthHost: r.AuthHost, AuthType: r.AuthType, Enabled: r.Enabled}
	}
	return out
}
