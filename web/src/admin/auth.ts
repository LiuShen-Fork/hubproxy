import { computed, ref } from 'vue'
import { adminApi, getToken, setToken, type User } from './api'

const user = ref<User | null>(null)
const loaded = ref(false)

export function useAuth() {
  const isAuthed = computed(() => !!user.value)
  const isAdmin = computed(() => user.value?.role === 'admin')

  async function bootstrap() {
    if (!getToken()) {
      user.value = null
      loaded.value = true
      return
    }
    try {
      const res = await adminApi.me()
      user.value = res.user
    } catch {
      setToken('')
      user.value = null
    } finally {
      loaded.value = true
    }
  }

  async function login(username: string, password: string) {
    const res = await adminApi.login(username, password)
    setToken(res.token)
    user.value = res.user
    return res.user
  }

  async function logout() {
    try {
      await adminApi.logout()
    } catch {
      /* ignore */
    }
    setToken('')
    user.value = null
  }

  function setUser(u: User | null) {
    user.value = u
  }

  return { user, loaded, isAuthed, isAdmin, bootstrap, login, logout, setUser }
}
