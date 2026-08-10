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
  ExternalLink,
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
  <div class="min-h-screen bg-muted/30">
    <div class="pointer-events-none fixed inset-0 -z-10 overflow-hidden">
      <div class="absolute -left-32 top-0 size-[22rem] rounded-full bg-primary/[0.06] blur-3xl" />
      <div class="absolute -right-24 bottom-20 size-[18rem] rounded-full bg-primary/[0.05] blur-3xl" />
    </div>

    <div class="flex min-h-screen">
      <aside
        :class="cn(
          'fixed inset-y-0 left-0 z-40 flex w-64 flex-col border-r border-border/70 bg-background/90 backdrop-blur-xl transition-transform lg:translate-x-0',
          open ? 'translate-x-0' : '-translate-x-full',
        )"
      >
        <div class="flex h-16 shrink-0 items-center gap-2.5 border-b border-border/70 px-4">
          <RouterLink
            to="/"
            class="group flex min-w-0 flex-1 items-center gap-2.5 rounded-xl px-1 py-1 transition-opacity hover:opacity-90"
            title="返回主页"
          >
            <div class="flex size-9 shrink-0 items-center justify-center rounded-xl bg-primary text-primary-foreground shadow-md shadow-primary/20">
              <Container class="size-4" />
            </div>
            <div class="min-w-0">
              <div class="flex items-center gap-1.5 truncate font-display text-sm font-semibold">
                {{ site.name }}
                <ExternalLink class="size-3 shrink-0 text-muted-foreground opacity-0 transition-opacity group-hover:opacity-100" />
              </div>
              <div class="truncate text-[11px] text-muted-foreground">
                {{ isAdmin && !isUserArea ? '管理员中控台' : '用户控制台' }}
              </div>
            </div>
          </RouterLink>
          <button class="rounded-lg p-1.5 text-muted-foreground hover:bg-accent lg:hidden" @click="open = false">
            <X class="size-5" />
          </button>
        </div>

        <div v-if="isAdmin" class="shrink-0 border-b border-border/70 p-3">
          <div class="grid grid-cols-2 gap-1 rounded-xl bg-muted/80 p-1">
            <button
              type="button"
              class="rounded-lg px-2 py-2 text-xs font-medium transition-all"
              :class="!isUserArea ? 'bg-background text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground'"
              @click="switchConsole('admin')"
            >
              管理
            </button>
            <button
              type="button"
              class="rounded-lg px-2 py-2 text-xs font-medium transition-all"
              :class="isUserArea ? 'bg-background text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground'"
              @click="switchConsole('user')"
            >
              用户
            </button>
          </div>
        </div>

        <nav class="min-h-0 flex-1 space-y-0.5 overflow-y-auto p-3">
          <RouterLink
            v-for="item in nav"
            :key="item.to"
            :to="item.to"
            :class="cn(
              'flex items-center gap-2.5 rounded-xl px-3 py-2.5 text-sm font-medium transition-all duration-150',
              isActive(item.to, item.exact)
                ? 'bg-primary text-primary-foreground shadow-sm shadow-primary/20'
                : 'text-muted-foreground hover:bg-accent/80 hover:text-accent-foreground',
            )"
            @click="open = false"
          >
            <component :is="item.icon" class="size-4 shrink-0 opacity-90" />
            {{ item.label }}
          </RouterLink>
        </nav>

        <div class="shrink-0 space-y-3 border-t border-border/70 p-4">
          <RouterLink
            to="/"
            class="flex items-center gap-2 rounded-xl border border-border/70 bg-muted/40 px-3 py-2 text-xs font-medium text-muted-foreground transition-colors hover:border-border hover:bg-accent hover:text-foreground"
          >
            <Home class="size-3.5" />
            返回主页
          </RouterLink>
          <div class="flex items-center gap-3">
            <div class="flex size-9 items-center justify-center rounded-full bg-primary/10 text-xs font-semibold text-primary">
              {{ (user?.username || '?').slice(0, 1).toUpperCase() }}
            </div>
            <div class="min-w-0 flex-1">
              <div class="truncate text-sm font-medium">{{ user?.username }}</div>
              <div class="truncate text-[11px] uppercase tracking-wide text-muted-foreground">{{ user?.role }}</div>
            </div>
          </div>
          <Button variant="outline" class="w-full rounded-xl" size="sm" @click="onLogout">
            <LogOut class="size-4" />
            退出登录
          </Button>
        </div>
      </aside>

      <div v-if="open" class="fixed inset-0 z-30 bg-black/40 backdrop-blur-[2px] lg:hidden" @click="open = false" />

      <div class="flex min-w-0 flex-1 flex-col lg:pl-64">
        <header class="sticky top-0 z-20 flex h-16 items-center gap-3 border-b border-border/70 bg-background/75 px-4 backdrop-blur-xl lg:px-6">
          <button class="rounded-lg p-1.5 text-muted-foreground hover:bg-accent lg:hidden" @click="open = true">
            <Menu class="size-5" />
          </button>
          <div class="min-w-0 flex-1">
            <div class="truncate font-display text-lg font-semibold tracking-tight">{{ route.meta.title || '控制台' }}</div>
          </div>
          <RouterLink
            to="/"
            class="inline-flex items-center gap-1.5 rounded-full border border-border/70 bg-background/80 px-3 py-1.5 text-xs font-medium text-muted-foreground transition-colors hover:border-border hover:text-foreground"
          >
            <Home class="size-3.5" />
            <span class="hidden sm:inline">主页</span>
          </RouterLink>
        </header>
        <main class="flex-1 overflow-y-auto p-4 lg:p-6">
          <RouterView />
        </main>
      </div>
    </div>
  </div>
</template>
