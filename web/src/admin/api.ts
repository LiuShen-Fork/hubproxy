const TOKEN_KEY = 'hubproxy_admin_token'

export function getToken(): string {
  return localStorage.getItem(TOKEN_KEY) || ''
}

export function setToken(token: string) {
  if (token) localStorage.setItem(TOKEN_KEY, token)
  else localStorage.removeItem(TOKEN_KEY)
}

export class ApiError extends Error {
  status: number
  code?: string
  constructor(message: string, status: number, code?: string) {
    super(message)
    this.status = status
    this.code = code
  }
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers || {})
  if (!headers.has('Content-Type') && init.body) {
    headers.set('Content-Type', 'application/json')
  }
  const token = getToken()
  if (token) headers.set('Authorization', `Bearer ${token}`)

  const res = await fetch(`/api/admin${path}`, { ...init, headers, credentials: 'same-origin' })
  const text = await res.text()
  let data: any = null
  try {
    data = text ? JSON.parse(text) : null
  } catch {
    data = { error: text }
  }
  if (!res.ok) {
    const msg =
      data?.error ||
      (typeof data === 'string' ? data : '') ||
      res.statusText ||
      '请求失败'
    // 旧进程未注册路由时会落到 GitHub 代理，返回纯文本「无效输入」
    if (res.status === 403 && (msg.includes('无效输入') || text.includes('无效输入'))) {
      throw new ApiError(
        '接口未找到（无效输入）。请停掉旧 hubproxy 进程后用最新代码重启：cd src; go run .',
        res.status,
        'STALE_SERVER',
      )
    }
    throw new ApiError(msg, res.status, data?.code)
  }
  return data as T
}

export interface User {
  id: number
  username: string
  role: 'admin' | 'user'
  must_change_password: boolean
  daily_pull_limit: number
  created_at: string
  updated_at: string
  last_login_at?: string
}

export interface UserQuota {
  daily_limit: number
  used_today: number
  remaining: number
  resets_at: string
  resets_at_human: string
}

export interface PullSession {
  id: string
  client_ip: string
  image_name: string
  registry: string
  tag: string
  category: string
  started_at: string
  last_seen_at: string
  completed_at?: string
  status: string
  bytes_total: number
  layer_count: number
  request_count: number
}

export interface DashboardStats {
  total_pulls: number
  total_bytes: number
  unique_ips: number
  active_pulls: number
  today_pulls: number
  today_bytes: number
  top_images: Array<{
    image_name: string
    registry: string
    category: string
    pull_count: number
    bytes_total: number
    unique_ips: number
  }>
  top_ips: Array<{
    client_ip: string
    pull_count: number
    bytes_total: number
    last_seen: string
  }>
  category_stats: Array<{ category: string; pull_count: number; bytes_total: number }>
  daily_trend: Array<{ day: string; pull_count: number; bytes_total: number }>
  recent_pulls: PullSession[]
}

export interface FeatureToggles {
  docker_hub: boolean
  github: boolean
  huggingface: boolean
  image_search: boolean
  offline_image: boolean
  public_mirror: boolean
}

export interface RegistryToggle {
  domain: string
  upstream: string
  auth_host: string
  auth_type: string
  enabled: boolean
  label: string
}

export interface AdminAccountSettings {
  form_register_enabled: boolean
  register_enabled?: boolean
  oauth_login_enabled: boolean
  oauth_register_enabled: boolean
  email_register_enabled?: boolean
}

export interface SiteSettingsApi {
  name: string
  full_name: string
  tagline: string
  description: string
  icp_text: string
  police_text: string
  announcement?: string
  project_name?: string
  project_url?: string
  icp_url?: string
  police_url?: string
}

export interface OAuthSettingsApi {
  enabled: boolean
  client_id: string
  client_secret: string
  auth_url: string
  token_url: string
  user_info_url: string
  scopes: string
  display_name: string
}

export interface EmailSettingsApi {
  enabled: boolean
  smtp_host: string
  smtp_port: number
  username: string
  password: string
  from: string
  from_name: string
  use_tls: boolean
}

export interface SettingsBundle {
  rate_limit: {
    request_limit: number
    period_hours: number
    pull_limit: number
  }
  security: { white_list: string[]; black_list: string[] }
  access: { white_list: string[]; black_list: string[]; proxy: string }
  admin: AdminAccountSettings
  pull_session: {
    window_minutes: number
    idle_minutes: number
    manifest_probe_seconds?: number
    re_pull_gap_seconds?: number
  }
  features: FeatureToggles
  registries: RegistryToggle[]
  site: SiteSettingsApi
  oauth: OAuthSettingsApi
  email?: EmailSettingsApi
  oauth_redirect_url?: string
}

export const adminApi = {
  publicConfig: () =>
    request<{
      register_enabled: boolean
      form_register_enabled: boolean
      oauth_login_enabled: boolean
      oauth_register_enabled: boolean
      oauth_bind_enabled: boolean
      email_register_enabled: boolean
      oauth: { enabled: boolean; ready: boolean; display_name: string }
      oauth_redirect_url?: string
      site: SiteSettingsApi
    }>('/public-config'),
  login: (username: string, password: string) =>
    request<{ token: string; user: User }>('/login', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    }),
  logout: () => request<{ ok: boolean }>('/logout', { method: 'POST' }),
  me: () => request<{ user: User }>('/me'),
  changePassword: (current_password: string, new_password: string) =>
    request<{ ok: boolean; token?: string; user?: User }>('/change-password', {
      method: 'POST',
      body: JSON.stringify({ current_password, new_password }),
    }),
  updateProfile: (body: {
    username?: string
    current_password?: string
    new_password?: string
  }) =>
    request<{ ok: boolean; token?: string; user?: User; message?: string }>('/profile', {
      method: 'POST',
      body: JSON.stringify(body),
    }),
  register: (username: string, password: string, email?: string, code?: string) =>
    request<{ user: User }>('/register', {
      method: 'POST',
      body: JSON.stringify({ username, password, email, code }),
    }),
  dashboard: (days = 14) => request<DashboardStats>(`/dashboard?days=${days}`),
  pulls: (q: Record<string, string | number | undefined>) => {
    const sp = new URLSearchParams()
    Object.entries(q).forEach(([k, v]) => {
      if (v !== undefined && v !== '') sp.set(k, String(v))
    })
    return request<{ items: PullSession[]; total: number; page: number; page_size: number }>(
      `/pulls?${sp}`,
    )
  },
  pull: (id: string) =>
    request<{ session: PullSession; events: any[] }>(`/pulls/${id}`),
  images: (q: Record<string, string | number | undefined>) => {
    const sp = new URLSearchParams()
    Object.entries(q).forEach(([k, v]) => {
      if (v !== undefined && v !== '') sp.set(k, String(v))
    })
    return request<{ items: any[]; total: number }>(`/images?${sp}`)
  },
  ips: (q: Record<string, string | number | undefined>) => {
    const sp = new URLSearchParams()
    Object.entries(q).forEach(([k, v]) => {
      if (v !== undefined && v !== '') sp.set(k, String(v))
    })
    return request<{ items: any[]; total: number }>(`/ips?${sp}`)
  },
  users: () => request<{ items: User[] }>('/users'),
  createUser: (body: { username: string; password: string; role?: string }) =>
    request<{ user: User }>('/users', { method: 'POST', body: JSON.stringify(body) }),
  updateUser: (id: number, body: { username?: string; role?: string; password?: string; daily_pull_limit?: number }) =>
    request<{ user: User }>(`/users/${id}`, { method: 'PATCH', body: JSON.stringify(body) }),
  deleteUser: (id: number) => request<{ ok: boolean }>(`/users/${id}`, { method: 'DELETE' }),
  settings: () => request<SettingsBundle>('/settings'),
  putRateLimit: (body: SettingsBundle['rate_limit']) =>
    request('/settings/rate-limit', { method: 'PUT', body: JSON.stringify(body) }),
  putSecurity: (body: SettingsBundle['security']) =>
    request('/settings/security', { method: 'PUT', body: JSON.stringify(body) }),
  putAccess: (body: SettingsBundle['access']) =>
    request('/settings/access', { method: 'PUT', body: JSON.stringify(body) }),
  putAdmin: (body: SettingsBundle['admin']) =>
    request('/settings/admin', { method: 'PUT', body: JSON.stringify(body) }),
  putSite: (body: SiteSettingsApi) =>
    request('/settings/site', { method: 'PUT', body: JSON.stringify(body) }),
  putOAuth: (body: OAuthSettingsApi) =>
    request('/settings/oauth', { method: 'PUT', body: JSON.stringify(body) }),
  putEmail: (body: EmailSettingsApi) =>
    request('/settings/email', { method: 'PUT', body: JSON.stringify(body) }),
  testEmail: (to?: string) =>
    request<{ ok: boolean; message?: string }>('/settings/email/test', {
      method: 'POST',
      body: JSON.stringify({ to: to || '' }),
    }),
  sendRegisterCode: (email: string) =>
    request<{ ok: boolean; message?: string }>('/register/send-code', {
      method: 'POST',
      body: JSON.stringify({ email }),
    }),
  putPullSession: (body: SettingsBundle['pull_session']) =>
    request('/settings/pull-session', { method: 'PUT', body: JSON.stringify(body) }),
  putFeatures: (body: FeatureToggles) =>
    request('/settings/features', { method: 'PUT', body: JSON.stringify(body) }),
  putRegistries: (body: RegistryToggle[]) =>
    request('/settings/registries', { method: 'PUT', body: JSON.stringify(body) }),
  oauthBindings: () =>
    request<{ items: Array<{ id: number; provider: string; subject: string; email?: string; display_name?: string }> }>(
      '/user/oauth/bindings',
    ),
  oauthUnbind: (provider: string) =>
    request<{ items: Array<{ id: number; provider: string; subject: string; email?: string; display_name?: string }> }>(
      `/user/oauth/bindings?provider=${encodeURIComponent(provider)}`,
      { method: 'DELETE' },
    ),
  addBlackIP: (ip: string) =>
    request('/security/blacklist', { method: 'POST', body: JSON.stringify({ ip }) }),
  removeBlackIP: (ip: string) =>
    request(`/security/blacklist?ip=${encodeURIComponent(ip)}`, { method: 'DELETE' }),
  addWhiteIP: (ip: string) =>
    request('/security/whitelist', { method: 'POST', body: JSON.stringify({ ip }) }),
  removeWhiteIP: (ip: string) =>
    request(`/security/whitelist?ip=${encodeURIComponent(ip)}`, { method: 'DELETE' }),

  // user console
  userDashboard: (days = 14) =>
    request<{ stats: DashboardStats; quota: UserQuota }>(`/user/dashboard?days=${days}`),
  userQuota: () => request<UserQuota>('/user/quota'),
  userPulls: (q: Record<string, string | number | undefined>) => {
    const sp = new URLSearchParams()
    Object.entries(q).forEach(([k, v]) => {
      if (v !== undefined && v !== '') sp.set(k, String(v))
    })
    return request<{ items: PullSession[]; total: number }>(`/user/pulls?${sp}`)
  },
  userToken: () =>
    request<{
      token: { token: string; user_id: number; status: string; created_at: string }
      examples: Record<string, string>
      require_token: boolean
      public_mirror?: boolean
    }>('/user/token'),
  resetUserToken: () =>
    request<{
      token: { token: string }
      examples: Record<string, string>
      message?: string
    }>('/user/token/reset', { method: 'POST' }),
  userIPWhitelist: () => request<{ items: string[] }>('/user/ip-whitelist'),
  setUserIPWhitelist: (items: string[]) =>
    request<{ items: string[] }>('/user/ip-whitelist', {
      method: 'PUT',
      body: JSON.stringify({ items }),
    }),
  addUserIP: (ip: string) =>
    request<{ items: string[] }>('/user/ip-whitelist', {
      method: 'POST',
      body: JSON.stringify({ ip }),
    }),
  removeUserIP: (ip: string) =>
    request<{ items: string[] }>(`/user/ip-whitelist?ip=${encodeURIComponent(ip)}`, {
      method: 'DELETE',
    }),
  userGuide: () =>
    request<{
      host: string
      token: string
      require_token: boolean
      public_mirror?: boolean
      examples: Record<string, string>
      notes: string[]
    }>('/user/guide'),
}

export function formatBytes(n: number): string {
  if (!n || n <= 0) return '0 B'
  const u = ['B', 'KB', 'MB', 'GB', 'TB']
  let i = 0
  let v = n
  while (v >= 1024 && i < u.length - 1) {
    v /= 1024
    i++
  }
  return `${v.toFixed(i === 0 ? 0 : 1)} ${u[i]}`
}

export function formatTime(iso?: string): string {
  if (!iso) return '-'
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  return d.toLocaleString()
}
