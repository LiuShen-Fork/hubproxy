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
	KeySite        = "site"
	KeyOAuth       = "oauth"
	KeyEmail       = "email"
)

const (
	FixedICPURL    = "https://beian.miit.gov.cn/"
	FixedPoliceURL = "https://beian.mps.gov.cn/#/query/webSearch?code="
	ProjectGitHub  = "https://github.com/LiuShen-Fork/hubproxy"
	ProjectName    = "HubProxy"
	// AuthorHomeURL is fixed project/author attribution (not admin-configurable).
	AuthorHomeURL = "https://www.liushen.fun/"
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
	// FormRegisterEnabled: allow username/password self-register on login page
	FormRegisterEnabled bool `json:"form_register_enabled"`
	// RegisterEnabled kept for backward compat (same as form_register_enabled)
	RegisterEnabled bool `json:"register_enabled"`
	// OAuthLoginEnabled: allow OAuth2 login for existing bindings
	OAuthLoginEnabled bool `json:"oauth_login_enabled"`
	// OAuthRegisterEnabled: allow creating new users via OAuth2
	OAuthRegisterEnabled bool `json:"oauth_register_enabled"`
	// EmailRegisterEnabled: require/use email verification for form registration
	EmailRegisterEnabled bool `json:"email_register_enabled"`
}

// FormRegisterAllowed is the effective form registration switch.
func (a AdminSettings) FormRegisterAllowed() bool {
	return a.FormRegisterEnabled || a.RegisterEnabled
}

// OAuthBindAllowed: bind is always on when OAuth provider is enabled.
func (a AdminSettings) OAuthBindAllowed() bool {
	return true
}

type SiteSettings struct {
	Name         string `json:"name"`
	FullName     string `json:"full_name"`
	Tagline      string `json:"tagline"`
	Description  string `json:"description"`
	ICPText      string `json:"icp_text"`
	PoliceText   string `json:"police_text"`
	Announcement string `json:"announcement"` // HTML, empty = no popup
}

type OAuthSettings struct {
	Enabled      bool   `json:"enabled"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	AuthURL      string `json:"auth_url"`
	TokenURL     string `json:"token_url"`
	UserInfoURL  string `json:"user_info_url"`
	Scopes       string `json:"scopes"`
	DisplayName  string `json:"display_name"`
}

type EmailSettings struct {
	Enabled  bool   `json:"enabled"`
	SMTPHost string `json:"smtp_host"`
	SMTPPort int    `json:"smtp_port"`
	Username string `json:"username"`
	Password string `json:"password"`
	From     string `json:"from"`
	FromName string `json:"from_name"`
	// UseTLS: STARTTLS on port 587 typically; if port 465 use implicit SSL
	UseTLS bool `json:"use_tls"`
}

func DefaultSiteSettings() SiteSettings {
	return SiteSettings{
		Name:         "镜像加速",
		FullName:     "自建多源镜像",
		Tagline:      "",
		Description:  "多源镜像加速服务，支持 Docker、GitHub、Hugging Face。",
		ICPText:      "",
		PoliceText:   "",
		Announcement: "",
	}
}

func DefaultOAuthSettings() OAuthSettings {
	return OAuthSettings{
		Enabled:     false,
		Scopes:      "openid profile email",
		DisplayName: "OAuth2 登录",
	}
}

func DefaultEmailSettings() EmailSettings {
	return EmailSettings{
		Enabled:  false,
		SMTPPort: 587,
		UseTLS:   true,
		FromName: "HubProxy",
	}
}

func DefaultAdminSettings() AdminSettings {
	return AdminSettings{
		FormRegisterEnabled:  false,
		RegisterEnabled:      false,
		OAuthLoginEnabled:    false,
		OAuthRegisterEnabled: false,
		EmailRegisterEnabled: false,
	}
}

// PoliceBeianURL builds fixed MPS query URL from police record number digits/code.
func PoliceBeianURL(policeText string) string {
	// extract trailing digits if possible, else use full text as code query
	code := ""
	for _, r := range policeText {
		if r >= '0' && r <= '9' {
			code += string(r)
		}
	}
	if code == "" {
		return "https://beian.mps.gov.cn/"
	}
	return FixedPoliceURL + code
}

type PullSessionSettings struct {
	// WindowMinutes: match uncounted session for same IP+image (manifest→blob glue)
	WindowMinutes int `json:"window_minutes"`
	// IdleMinutes: mark counted active sessions completed after idle
	IdleMinutes int `json:"idle_minutes"`
	// ManifestProbeSeconds: delete manifest-only sessions with no blob after this idle
	ManifestProbeSeconds int `json:"manifest_probe_seconds"`
	// RePullGapSeconds kept for UI/API compatibility (ignored: re-pull always new after first blob)
	RePullGapSeconds int `json:"re_pull_gap_seconds,omitempty"`
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
		KeyAdmin:       DefaultAdminSettings(),
		KeySite:        DefaultSiteSettings(),
		KeyOAuth:       DefaultOAuthSettings(),
		KeyEmail:       DefaultEmailSettings(),
		KeyPullSession: PullSessionSettings{
			WindowMinutes:        15,
			IdleMinutes:          30,
			ManifestProbeSeconds: 60,
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
	def := DefaultAdminSettings()
	var s AdminSettings
	if err := GetSetting(KeyAdmin, &s); err != nil {
		return def
	}
	// sync legacy register_enabled → form_register_enabled
	if s.RegisterEnabled && !s.FormRegisterEnabled {
		s.FormRegisterEnabled = true
	}
	if s.FormRegisterEnabled {
		s.RegisterEnabled = true
	}
	return s
}

func LoadSite() SiteSettings {
	def := DefaultSiteSettings()
	var s SiteSettings
	if err := GetSetting(KeySite, &s); err != nil {
		return def
	}
	if s.Name == "" {
		s.Name = def.Name
	}
	if s.FullName == "" {
		s.FullName = def.FullName
	}
	return s
}

func LoadOAuth() OAuthSettings {
	def := DefaultOAuthSettings()
	var s OAuthSettings
	if err := GetSetting(KeyOAuth, &s); err != nil {
		return def
	}
	if s.Scopes == "" {
		s.Scopes = def.Scopes
	}
	if s.DisplayName == "" {
		s.DisplayName = def.DisplayName
	}
	// migrate legacy github provider endpoints
	var raw map[string]any
	if GetSetting(KeyOAuth, &raw) == nil {
		if p, ok := raw["provider"].(string); ok && p == "github" {
			if s.AuthURL == "" {
				s.AuthURL = "https://github.com/login/oauth/authorize"
			}
			if s.TokenURL == "" {
				s.TokenURL = "https://github.com/login/oauth/access_token"
			}
			if s.UserInfoURL == "" {
				s.UserInfoURL = "https://api.github.com/user"
			}
			if s.Scopes == "openid profile email" {
				s.Scopes = "read:user user:email"
			}
		}
	}
	return s
}

func LoadEmail() EmailSettings {
	def := DefaultEmailSettings()
	var s EmailSettings
	if err := GetSetting(KeyEmail, &s); err != nil {
		return def
	}
	if s.SMTPPort <= 0 {
		s.SMTPPort = 587
	}
	return s
}

// PublicOAuthView is safe to expose to browsers (no secrets).
func (o OAuthSettings) PublicView() map[string]any {
	ready := o.Enabled && o.ClientID != "" && o.ClientSecret != "" &&
		o.AuthURL != "" && o.TokenURL != "" && o.UserInfoURL != ""
	return map[string]any{
		"enabled":      o.Enabled && ready,
		"ready":        ready,
		"display_name": o.DisplayName,
	}
}

// PublicSiteView builds public site payload with fixed URLs.
func (s SiteSettings) PublicSiteView() map[string]any {
	icpURL := ""
	if s.ICPText != "" {
		icpURL = FixedICPURL
	}
	policeURL := ""
	if s.PoliceText != "" {
		policeURL = PoliceBeianURL(s.PoliceText)
	}
	return map[string]any{
		"name":         s.Name,
		"full_name":    s.FullName,
		"tagline":      s.Tagline,
		"description":  s.Description,
		"author_home":  AuthorHomeURL,
		"project_name": ProjectName,
		"project_url":  ProjectGitHub,
		"icp_text":     s.ICPText,
		"icp_url":      icpURL,
		"police_text":  s.PoliceText,
		"police_url":   policeURL,
		"announcement": s.Announcement,
	}
}

func LoadPullSession() PullSessionSettings {
	var s PullSessionSettings
	if err := GetSetting(KeyPullSession, &s); err != nil || s.WindowMinutes <= 0 {
		return PullSessionSettings{WindowMinutes: 15, IdleMinutes: 30, ManifestProbeSeconds: 60}
	}
	if s.IdleMinutes <= 0 {
		s.IdleMinutes = 30
	}
	if s.ManifestProbeSeconds <= 0 {
		// migrate old re_pull_gap or default 60s
		if s.RePullGapSeconds > 0 && s.RePullGapSeconds < 600 {
			s.ManifestProbeSeconds = 60
		} else {
			s.ManifestProbeSeconds = 60
		}
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
