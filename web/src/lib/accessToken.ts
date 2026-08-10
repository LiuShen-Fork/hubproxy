export const ACCESS_TOKEN_KEY = 'hubproxy_access_token'

export function getAccessToken(): string {
  return localStorage.getItem(ACCESS_TOKEN_KEY) || ''
}

export function setAccessToken(token: string) {
  const clean = token.trim()
  if (clean) localStorage.setItem(ACCESS_TOKEN_KEY, clean)
  else localStorage.removeItem(ACCESS_TOKEN_KEY)
}
