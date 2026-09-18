import { createApp } from 'vue'
import './style.css'
import App from './App.vue'
import router from './router'
import { loadPublicSite } from './lib/site'
import { setToken } from './admin/api'

if ('scrollRestoration' in history) {
  history.scrollRestoration = 'manual'
}

// OAuth callback may redirect with #oauth_token=<token>&welcome=1
if (typeof window !== 'undefined' && window.location.hash.startsWith('#oauth_token=')) {
  const params = new URLSearchParams(window.location.hash.slice(1))
  const token = params.get('oauth_token') || ''
  if (token) {
    setToken(token)
  }
  // 第三方首次登录时后端自动建号，落地后由 AdminLayout 弹一次提示
  if (params.get('welcome') === '1') {
    sessionStorage.setItem('hubproxy_oauth_welcome', '1')
  }
  history.replaceState(null, '', window.location.pathname + window.location.search)
}

loadPublicSite().finally(() => {
  createApp(App).use(router).mount('#app')
})
