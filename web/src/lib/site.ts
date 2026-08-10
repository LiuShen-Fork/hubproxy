import { reactive, readonly } from 'vue'

export type SiteInfo = {
  name: string
  fullName: string
  tagline: string
  description: string
  authorHome: string
  projectName: string
  projectUrl: string
  icp: { text: string; href: string }
  police: { text: string; href: string }
  announcement: string
}

const defaults: SiteInfo = {
  name: '镜像加速',
  fullName: '自建多源镜像',
  tagline: '',
  description: '多源镜像加速服务，支持 Docker、GitHub、Hugging Face。',
  authorHome: '',
  projectName: 'HubProxy',
  projectUrl: 'https://github.com/LiuShen-Fork/hubproxy',
  icp: { text: '', href: '' },
  police: { text: '', href: '' },
  announcement: '',
}

const state = reactive<SiteInfo>({
  ...defaults,
  icp: { ...defaults.icp },
  police: { ...defaults.police },
})

export const site = readonly(state)

export function applySiteFromApi(raw: any) {
  if (!raw || typeof raw !== 'object') return
  if (raw.name) state.name = String(raw.name)
  if (raw.full_name) state.fullName = String(raw.full_name)
  if (raw.tagline != null) state.tagline = String(raw.tagline)
  if (raw.description != null) state.description = String(raw.description)
  if (raw.author_home != null) state.authorHome = String(raw.author_home)
  if (raw.project_name) state.projectName = String(raw.project_name)
  if (raw.project_url) state.projectUrl = String(raw.project_url)
  if (raw.icp_text != null) state.icp.text = String(raw.icp_text)
  if (raw.icp_url != null) state.icp.href = String(raw.icp_url)
  if (raw.police_text != null) state.police.text = String(raw.police_text)
  if (raw.police_url != null) state.police.href = String(raw.police_url)
  if (raw.announcement != null) state.announcement = String(raw.announcement)
  if (state.name) document.title = state.name
}

export async function loadPublicSite() {
  try {
    const res = await fetch('/api/admin/public-config')
    if (!res.ok) return
    const data = await res.json()
    if (data.site) applySiteFromApi(data.site)
  } catch {
    /* defaults */
  }
}
