package db

import (
	"sync"

	"hubproxy/config"
)

// Runtime holds hot-reloadable settings sourced from SQLite (with config.toml seed).
type Runtime struct {
	mu          sync.RWMutex
	RateLimit   RateLimitSettings
	Security    SecuritySettings
	Access      AccessSettings
	Admin       AdminSettings
	PullSession PullSessionSettings
	Features    FeatureToggles
	Registries  []RegistryToggle
}

var GlobalRuntime = &Runtime{}

func (r *Runtime) Reload() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.RateLimit = LoadRateLimit()
	r.Security = LoadSecurity()
	r.Access = LoadAccess()
	r.Admin = LoadAdmin()
	r.PullSession = LoadPullSession()
	r.Features = LoadFeatures()
	r.Registries = LoadRegistries()
}

func (r *Runtime) Snapshot() (RateLimitSettings, SecuritySettings, AccessSettings, AdminSettings, PullSessionSettings) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.RateLimit, r.Security, r.Access, r.Admin, r.PullSession
}

func (r *Runtime) GetRateLimit() RateLimitSettings {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.RateLimit
}

func (r *Runtime) GetSecurity() SecuritySettings {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.Security
}

func (r *Runtime) GetAccess() AccessSettings {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.Access
}

func (r *Runtime) GetAdmin() AdminSettings {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.Admin
}

func (r *Runtime) GetPullSession() PullSessionSettings {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.PullSession
}

func (r *Runtime) GetFeatures() FeatureToggles {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.Features
}

func (r *Runtime) GetRegistries() []RegistryToggle {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]RegistryToggle, len(r.Registries))
	copy(out, r.Registries)
	return out
}

func (r *Runtime) GetRegistry(domain string) (RegistryToggle, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, reg := range r.Registries {
		if reg.Domain == domain {
			return reg, true
		}
	}
	return RegistryToggle{}, false
}

func SeedFromConfig(cfg *config.AppConfig) error {
	rate := RateLimitSettings{
		RequestLimit: cfg.RateLimit.RequestLimit,
		PeriodHours:  cfg.RateLimit.PeriodHours,
		PullLimit:    50,
	}
	security := SecuritySettings{
		WhiteList: append([]string(nil), cfg.Security.WhiteList...),
		BlackList: append([]string(nil), cfg.Security.BlackList...),
	}
	access := AccessSettings{
		WhiteList: append([]string(nil), cfg.Access.WhiteList...),
		BlackList: append([]string(nil), cfg.Access.BlackList...),
		Proxy:     cfg.Access.Proxy,
	}
	if err := EnsureDefaultSettings(rate, security, access); err != nil {
		return err
	}
	// seed registries from config.toml if DB empty of extras
	if len(cfg.Registries) > 0 {
		list := LoadRegistries()
		by := map[string]int{}
		for i, r := range list {
			by[r.Domain] = i
		}
		changed := false
		for domain, m := range cfg.Registries {
			if idx, ok := by[domain]; ok {
				list[idx].Enabled = m.Enabled
				list[idx].Upstream = m.Upstream
				list[idx].AuthHost = m.AuthHost
				list[idx].AuthType = m.AuthType
				changed = true
			} else {
				list = append(list, RegistryToggle{
					Domain: domain, Upstream: m.Upstream, AuthHost: m.AuthHost,
					AuthType: m.AuthType, Enabled: m.Enabled, Label: domain,
				})
				changed = true
			}
		}
		if changed {
			_ = SetSetting(KeyRegistries, list)
		}
	}
	GlobalRuntime.Reload()
	return nil
}

func ApplyAccessToController() {
	access := GlobalRuntime.GetAccess()
	if OnAccessUpdated != nil {
		OnAccessUpdated(access)
	}
}

// SyncAccessProxy writes proxy setting into process env bridge used by HTTP clients.
func SyncAccessProxy(proxy string) {
	if OnProxyUpdated != nil {
		OnProxyUpdated(proxy)
	}
}

func ApplySecurityToLimiter() {
	if OnSecurityUpdated != nil {
		OnSecurityUpdated(GlobalRuntime.GetSecurity(), GlobalRuntime.GetRateLimit())
	}
}

// Callbacks wired from main/utils to avoid import cycles.
var (
	OnAccessUpdated   func(AccessSettings)
	OnSecurityUpdated func(SecuritySettings, RateLimitSettings)
	OnProxyUpdated    func(proxy string)
)
