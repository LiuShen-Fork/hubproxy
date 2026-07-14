<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { RouterLink, useRouter } from 'vue-router'
import { ArrowLeft, Container, Loader2, ShieldCheck } from 'lucide-vue-next'
import Button from '@/components/ui/Button.vue'
import Input from '@/components/ui/Input.vue'
import Label from '@/components/ui/Label.vue'
import Card from '@/components/ui/Card.vue'
import CardContent from '@/components/ui/CardContent.vue'
import CardHeader from '@/components/ui/CardHeader.vue'
import CardTitle from '@/components/ui/CardTitle.vue'
import { adminApi } from '../api'
import { useAuth } from '../auth'
import { site } from '@/lib/site'

const router = useRouter()
const { login } = useAuth()
const username = ref('')
const password = ref('')
const error = ref('')
const loading = ref(false)
const registerEnabled = ref(false)
const mode = ref<'login' | 'register'>('login')

onMounted(async () => {
  try {
    const cfg = await adminApi.publicConfig()
    registerEnabled.value = cfg.register_enabled
  } catch {
    /* ignore */
  }
})

async function submit() {
  error.value = ''
  loading.value = true
  try {
    if (mode.value === 'register') {
      await adminApi.register(username.value, password.value)
      mode.value = 'login'
      error.value = '注册成功，请登录'
      return
    }
    const user = await login(username.value, password.value)
    if (user.must_change_password) {
      await router.replace('/admin/change-password')
    } else {
      await router.replace(user.role === 'admin' ? '/admin' : '/admin/user')
    }
  } catch (e: any) {
    error.value = e?.message || '登录失败'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="relative flex min-h-screen flex-col overflow-hidden">
    <div class="pointer-events-none absolute inset-0 -z-10">
      <div class="absolute inset-0 bg-background" />
      <div class="absolute -left-24 top-0 size-[28rem] rounded-full bg-primary/10 blur-3xl" />
      <div class="absolute -right-20 bottom-0 size-[24rem] rounded-full bg-primary/8 blur-3xl" />
      <div class="absolute inset-0 bg-[radial-gradient(ellipse_at_top,transparent_20%,var(--background)_70%)]" />
    </div>

    <header class="flex items-center justify-between px-5 py-4 sm:px-8">
      <RouterLink
        to="/"
        class="inline-flex items-center gap-2 rounded-full border border-border/70 bg-background/70 px-3.5 py-2 text-sm text-muted-foreground backdrop-blur-md transition-colors hover:border-border hover:text-foreground"
      >
        <ArrowLeft class="size-4" />
        返回主页
      </RouterLink>
      <div class="hidden items-center gap-2 text-xs text-muted-foreground sm:flex">
        <ShieldCheck class="size-3.5 text-primary" />
        {{ site.fullName }}
      </div>
    </header>

    <div class="flex flex-1 items-center justify-center px-4 pb-12">
      <Card class="w-full max-w-md border-border/70 bg-background/80 shadow-xl shadow-black/5 backdrop-blur-xl dark:shadow-black/20">
        <CardHeader class="items-center space-y-3 pb-2 text-center">
          <div class="relative">
            <div class="flex size-14 items-center justify-center rounded-2xl bg-primary text-primary-foreground shadow-lg shadow-primary/25">
              <Container class="size-6" />
            </div>
            <div class="absolute -inset-2 -z-10 rounded-3xl bg-primary/15 blur-xl" />
          </div>
          <div class="space-y-1.5">
            <CardTitle class="font-display text-2xl tracking-tight">{{ site.name }}</CardTitle>
            <p class="text-sm text-muted-foreground">
              {{ mode === 'login' ? '登录控制台，管理镜像加速与访问策略' : '创建账号以使用个人访问令牌' }}
            </p>
          </div>
        </CardHeader>
        <CardContent class="pt-2">
          <form class="space-y-4" @submit.prevent="submit">
            <div class="space-y-2">
              <Label for="username">用户名</Label>
              <Input
                id="username"
                v-model="username"
                autocomplete="username"
                placeholder="请输入用户名"
                required
              />
            </div>
            <div class="space-y-2">
              <Label for="password">密码</Label>
              <Input
                id="password"
                v-model="password"
                type="password"
                autocomplete="current-password"
                placeholder="请输入密码"
                required
              />
            </div>
            <p
              v-if="error"
              class="rounded-lg px-3 py-2 text-sm"
              :class="error.includes('成功') ? 'bg-emerald-500/10 text-emerald-700 dark:text-emerald-300' : 'bg-destructive/10 text-destructive'"
            >
              {{ error }}
            </p>
            <Button class="w-full h-11 rounded-xl" :disabled="loading" type="submit">
              <Loader2 v-if="loading" class="size-4 animate-spin" />
              {{ mode === 'login' ? '登录' : '注册' }}
            </Button>
            <button
              v-if="registerEnabled"
              type="button"
              class="w-full text-center text-sm text-muted-foreground transition-colors hover:text-foreground"
              @click="mode = mode === 'login' ? 'register' : 'login'"
            >
              {{ mode === 'login' ? '没有账号？注册' : '已有账号？登录' }}
            </button>
          </form>
          <p class="mt-6 text-center text-[11px] leading-relaxed text-muted-foreground/80">
            首次登录后请立即修改默认密码 · 会话将在超时后自动失效
          </p>
        </CardContent>
      </Card>
    </div>
  </div>
</template>
