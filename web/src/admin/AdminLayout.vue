<script setup lang="ts">
import { computed, ref } from 'vue'
import { RouterLink, RouterView, useRoute, useRouter } from 'vue-router'
import {
  LayoutDashboard,
  Package,
  Network,
  Shield,
  Users,
  Settings,
  LogOut,
  Container,
  Menu,
  X,
  UserRound,
  KeyRound,
  ToggleLeft,
  Home,
} from 'lucide-vue-next'
import { useAuth } from './auth'
import Button from '@/components/ui/Button.vue'
import { cn } from '@/lib/utils'
import { site } from '@/lib/site'

const route = useRoute()
const router = useRouter()
const { user, logout, isAdmin } = useAuth()
const open = ref(false)

// NOTE: must not match /admin/users (admin user management)
const isUserArea = computed(() => {
  const p = route.path
  return (
    p === '/admin/user' ||
    p.startsWith('/admin/user/') ||
    p === '/admin/change-password'
  )
})

const adminNav = [
  { to: '/admin', label: '全局大屏', icon: LayoutDashboard, exact: true },
  { to: '/admin/pulls', label: '全局拉取', icon: Package },
  { to: '/admin/images', label: '镜像统计', icon: Container },
  { to: '/admin/ips', label: 'IP 分析', icon: Network },
  { to: '/admin/features', label: '功能开关', icon: ToggleLeft },
  { to: '/admin/security', label: '安全限流', icon: Shield },
  { to: '/admin/users', label: '用户管理', icon: Users },
  { to: '/admin/settings', label: '系统设置', icon: Settings },
]

const userNav = [
  { to: '/admin/user', label: '我的概览', icon: Home, exact: true },
  { to: '/admin/user/token', label: '访问令牌', icon: KeyRound },
  { to: '/admin/user/pulls', label: '我的拉取', icon: Package },
  { to: '/admin/user/ip', label: 'IP 白名单', icon: Network },
  { to: '/admin/change-password', label: '账户资料', icon: UserRound },
]

const nav = computed(() => {
  if (isAdmin.value && !isUserArea.value) {
    return adminNav
  }
  return userNav
})

async function onLogout() {
  await logout()
  router.push('/admin/login')
}

function isActive(to: string, exact?: boolean) {
  if (exact) return route.path === to
  return route.path === to || route.path.startsWith(to + '/')
}

function switchConsole(mode: 'admin' | 'user') {
  router.push(mode === 'admin' ? '/admin' : '/admin/user')
  open.value = false
}
</script>

<template>
  <div class="min-h-screen bg-muted/40">
    <div class="flex min-h-screen">
      <aside
        :class="cn(
          'fixed inset-y-0 left-0 z-40 flex w-64 flex-col border-r border-border bg-background transition-transform lg:translate-x-0',
          open ? 'translate-x-0' : '-translate-x-full',
        )"
      >
        <div class="flex h-16 shrink-0 items-center gap-2 border-b border-border px-5">
          <div class="flex size-9 items-center justify-center rounded-lg bg-primary text-primary-foreground">
            <Container class="size-4" />
          </div>
          <div class="min-w-0">
            <div class="truncate font-display text-sm font-semibold">{{ site.name }}</div>
            <div class="truncate text-xs text-muted-foreground">
              {{ isAdmin && !isUserArea ? '管理员中控台' : '用户控制台' }}
            </div>
          </div>
          <button class="ml-auto lg:hidden" @click="open = false">
            <X class="size-5" />
          </button>
        </div>

        <div v-if="isAdmin" class="shrink-0 border-b border-border p-3">
          <div class="grid grid-cols-2 gap-1 rounded-lg bg-muted p-1">
            <button
              type="button"
              class="rounded-md px-2 py-1.5 text-xs font-medium transition-colors"
              :class="!isUserArea ? 'bg-background shadow-sm' : 'text-muted-foreground'"
              @click="switchConsole('admin')"
            >
              管理
            </button>
            <button
              type="button"
              class="rounded-md px-2 py-1.5 text-xs font-medium transition-colors"
              :class="isUserArea ? 'bg-background shadow-sm' : 'text-muted-foreground'"
              @click="switchConsole('user')"
            >
              用户
            </button>
          </div>
        </div>

        <nav class="min-h-0 flex-1 space-y-1 overflow-y-auto p-3">
          <RouterLink
            v-for="item in nav"
            :key="item.to"
            :to="item.to"
            :class="cn(
              'flex items-center gap-2.5 rounded-lg px-3 py-2.5 text-sm font-medium transition-colors',
              isActive(item.to, item.exact)
                ? 'bg-primary text-primary-foreground'
                : 'text-muted-foreground hover:bg-accent hover:text-accent-foreground',
            )"
            @click="open = false"
          >
            <component :is="item.icon" class="size-4 shrink-0" />
            {{ item.label }}
          </RouterLink>
        </nav>

        <div class="shrink-0 border-t border-border p-4">
          <div class="mb-3 truncate text-sm">
            <div class="font-medium">{{ user?.username }}</div>
            <div class="text-xs text-muted-foreground">{{ user?.role }}</div>
          </div>
          <Button variant="outline" class="w-full" size="sm" @click="onLogout">
            <LogOut class="size-4" />
            退出登录
          </Button>
        </div>
      </aside>

      <div v-if="open" class="fixed inset-0 z-30 bg-black/40 lg:hidden" @click="open = false" />

      <div class="flex min-w-0 flex-1 flex-col lg:pl-64">
        <header class="sticky top-0 z-20 flex h-16 items-center gap-3 border-b border-border bg-background/90 px-4 backdrop-blur lg:px-6">
          <button class="lg:hidden" @click="open = true">
            <Menu class="size-5" />
          </button>
          <div class="min-w-0">
            <div class="truncate font-display text-lg font-semibold">{{ route.meta.title || '控制台' }}</div>
          </div>
        </header>
        <main class="flex-1 overflow-y-auto p-4 lg:p-6">
          <RouterView />
        </main>
      </div>
    </div>
  </div>
</template>
