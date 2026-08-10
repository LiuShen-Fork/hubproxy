import { createRouter, createWebHistory } from 'vue-router'
import HomePage from '@/pages/HomePage.vue'
import ImagesPage from '@/pages/ImagesPage.vue'
import SearchPage from '@/pages/SearchPage.vue'
import AdminLayout from '@/admin/AdminLayout.vue'
import LoginPage from '@/admin/pages/LoginPage.vue'
import ChangePasswordPage from '@/admin/pages/ChangePasswordPage.vue'
import DashboardPage from '@/admin/pages/DashboardPage.vue'
import PullsPage from '@/admin/pages/PullsPage.vue'
import AdminImagesPage from '@/admin/pages/ImagesPage.vue'
import IPsPage from '@/admin/pages/IPsPage.vue'
import SecurityPage from '@/admin/pages/SecurityPage.vue'
import UsersPage from '@/admin/pages/UsersPage.vue'
import SettingsPage from '@/admin/pages/SettingsPage.vue'
import FeaturesPage from '@/admin/pages/FeaturesPage.vue'
import UserDashboardPage from '@/admin/pages/UserDashboardPage.vue'
import UserTokenPage from '@/admin/pages/UserTokenPage.vue'
import UserPullsPage from '@/admin/pages/UserPullsPage.vue'
import UserIPPage from '@/admin/pages/UserIPPage.vue'
import { getToken } from '@/admin/api'
import { useAuth } from '@/admin/auth'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/',
      component: HomePage,
      meta: { title: '镜像加速' },
    },
    {
      path: '/images',
      component: ImagesPage,
      meta: { title: '离线镜像下载' },
    },
    {
      path: '/search',
      component: SearchPage,
      meta: { title: '镜像搜索' },
    },
    {
      path: '/admin/login',
      component: LoginPage,
      meta: { title: '登录', public: true },
    },
    {
      path: '/admin',
      component: AdminLayout,
      meta: { requiresAuth: true },
      children: [
        // admin console
        {
          path: '',
          component: DashboardPage,
          meta: { title: '全局大屏', requiresAuth: true, requiresAdmin: true },
        },
        {
          path: 'pulls',
          component: PullsPage,
          meta: { title: '全局拉取', requiresAuth: true, requiresAdmin: true },
        },
        {
          path: 'images',
          component: AdminImagesPage,
          meta: { title: '镜像统计', requiresAuth: true, requiresAdmin: true },
        },
        {
          path: 'ips',
          component: IPsPage,
          meta: { title: 'IP 分析', requiresAuth: true, requiresAdmin: true },
        },
        {
          path: 'features',
          component: FeaturesPage,
          meta: { title: '功能开关', requiresAuth: true, requiresAdmin: true },
        },
        {
          path: 'security',
          component: SecurityPage,
          meta: { title: '安全限流', requiresAuth: true, requiresAdmin: true },
        },
        {
          path: 'users',
          component: UsersPage,
          meta: { title: '用户管理', requiresAuth: true, requiresAdmin: true },
        },
        {
          path: 'settings',
          component: SettingsPage,
          meta: { title: '系统设置', requiresAuth: true, requiresAdmin: true },
        },
        // user console
        {
          path: 'user',
          component: UserDashboardPage,
          meta: { title: '我的概览', requiresAuth: true },
        },
        {
          path: 'user/token',
          component: UserTokenPage,
          meta: { title: '访问令牌', requiresAuth: true },
        },
        {
          path: 'user/pulls',
          component: UserPullsPage,
          meta: { title: '我的拉取', requiresAuth: true },
        },
        {
          path: 'user/ip',
          component: UserIPPage,
          meta: { title: 'IP 白名单', requiresAuth: true },
        },
        {
          path: 'change-password',
          component: ChangePasswordPage,
          meta: { title: '账户资料', requiresAuth: true },
        },
      ],
    },
  ],
  scrollBehavior(to, from, savedPosition) {
    if (savedPosition) return savedPosition
    if (to.path !== from.path) return { top: 0, left: 0 }
    return false
  },
})

router.beforeEach(async (to) => {
  if (!to.path.startsWith('/admin')) return true

  const auth = useAuth()
  if (!auth.loaded.value) {
    await auth.bootstrap()
  }

  if (to.meta.public) {
    if (getToken() && auth.user.value && !auth.user.value.must_change_password) {
      return auth.user.value.role === 'admin' ? '/admin' : '/admin/user'
    }
    return true
  }

  if (!getToken() || !auth.user.value) {
    return { path: '/admin/login', query: { redirect: to.fullPath } }
  }

  if (auth.user.value.must_change_password && to.path !== '/admin/change-password') {
    return '/admin/change-password'
  }

  if (to.meta.requiresAdmin && auth.user.value.role !== 'admin') {
    return '/admin/user'
  }

  // plain users landing on /admin root
  if (to.path === '/admin' && auth.user.value.role !== 'admin') {
    return '/admin/user'
  }

  return true
})

router.afterEach((to) => {
  const title = (to.meta.title as string) || '清羽镜像'
  document.title = `${title} · 清羽镜像`
})

export default router
