import { createApp } from 'vue'
import './style.css'
import App from './App.vue'
import router from './router'
import { loadPublicSite } from './lib/site'
import { setToken } from './admin/api'

if ('scrollRestoration' in history) {
  history.scrollRestoration = 'manual'
}

// OAuth callback may redirect with #oauth_token=
if (typeof window !== 'undefined' && window.location.hash.startsWith('#oauth_token=')) {
  const token = decodeURIComponent(window.location.hash.slice('#oauth_token='.length))
  if (token) {
    setToken(token)
    history.replaceState(null, '', window.location.pathname + window.location.search)
  }
}

loadPublicSite().finally(() => {
  createApp(App).use(router).mount('#app')
})
