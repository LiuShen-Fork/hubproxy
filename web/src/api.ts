class ApiError extends Error {
  status: number

  constructor(message: string, status: number) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

async function parseError(res: Response): Promise<string> {
  const contentType = res.headers.get('Content-Type') || ''
  if (contentType.includes('application/json')) {
    try {
      const data = (await res.json()) as { error?: string; message?: string }
      return data.error || data.message || `请求失败 (${res.status})`
    } catch {
      return `请求失败 (${res.status})`
    }
  }
  try {
    const text = await res.text()
    return text || `请求失败 (${res.status})`
  } catch {
    return `请求失败 (${res.status})`
  }
}

async function getJSON<T>(url: string, init?: RequestInit): Promise<T> {
  const res = await fetch(url, {
    ...init,
    headers: {
      Accept: 'application/json',
      ...(init?.headers || {}),
    },
    cache: 'no-store',
  })
  if (!res.ok) throw new ApiError(await parseError(res), res.status)
  return (await res.json()) as T
}

export interface Repository {
  repo_name?: string
  short_description?: string
  is_official?: boolean
  star_count?: number
  pull_count?: number
  namespace?: string
}

export interface SearchResponse {
  count: number
  results: Repository[]
}

export interface TagInfo {
  name: string
  last_updated?: string
  full_size?: number
  images?: Array<{
    architecture?: string
    os?: string
    variant?: string
    size?: number
  }>
}

export interface TagPageResult {
  tags: TagInfo[]
  has_more: boolean
}

export function searchImages(q: string, page: number, pageSize = 25) {
  const params = new URLSearchParams({
    q,
    page: String(page),
    page_size: String(pageSize),
  })
  return getJSON<SearchResponse>(`/api/search?${params}`)
}

export function fetchTags(namespace: string, name: string, page: number, pageSize = 100) {
  const params = new URLSearchParams({
    page: String(page),
    page_size: String(pageSize),
  })
  return getJSON<TagPageResult>(
    `/api/tags/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}?${params}`,
  )
}
