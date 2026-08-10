<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
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
import { applySiteFromApi, site } from '@/lib/site'
import { toastError, toastSuccess } from '@/lib/toast'

const router = useRouter()
const route = useRoute()
const { login } = useAuth()
const username = ref('')
const password = ref('')
const email = ref('')
const code = ref('')
const error = ref('')
const loading = ref(false)
const sending = ref(false)
const registerEnabled = ref(false)
const emailRegister = ref(false)
const oauthLogin = ref(false)
const oauthLabel = ref('OAuth2 登录')
const mode = ref<'login' | 'register'>('login')

onMounted(async () => {
  const qErr = route.query.oauth_error
  if (typeof qErr === 'string' && qErr) {
    error.value = qErr
    toastError(qErr)
  }
  try {
    const cfg = await adminApi.publicConfig()
    registerEnabled.value = !!(cfg.form_register_enabled ?? cfg.register_enabled)
    emailRegister.value = !!cfg.email_register_enabled
    oauthLogin.value = !!(cfg.oauth_login_enabled && cfg.oauth?.enabled)
    if (cfg.oauth?.display_name) oauthLabel.value = cfg.oauth.display_name
    if (cfg.site) applySiteFromApi(cfg.site)
  } catch {
    /* ignore */
  }
})

function startOAuth() {
  window.location.href = '/api/admin/oauth/start?mode=login'
}

async function sendCode() {
  if (!email.value.trim()) {
    toastError('请先填写邮箱')
    return
  }
  sending.value = true
  try {
    const res = await adminApi.sendRegisterCode(email.value.trim())
    toastSuccess(res.message || '验证码已发送')
  } catch (e: any) {
    toastError(e?.message || '发送失败')
  } finally {
    sending.value = false
  }
}

async function submit() {
  error.value = ''
  loading.value = true
  try {
    if (mode.value === 'register') {
      await adminApi.register(
        username.value,
        password.value,
        emailRegister.value ? email.value : undefined,
        emailRegister.value ? code.value : undefined,
      )
      mode.value = 'login'
      toastSuccess('注册成功，请登录')
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
    toastError(e?.message || '登录失败')
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
        {{ site.fullName || site.name }}
      </div>
    </header>

    <div class="flex flex-1 items-center justify-center px-4 pb-12">
      <Card class="w-full max-w-md border-border/70 bg-background/80 shadow-xl backdrop-blur-xl">
        <CardHeader class="items-center space-y-3 pb-2 text-center">
          <div class="flex size-14 items-center justify-center rounded-2xl bg-primary text-primary-foreground shadow-lg shadow-primary/25">
            <Container class="size-6" />
          </div>
          <div class="space-y-1.5">
            <CardTitle class="font-display text-2xl tracking-tight">{{ site.name }}</CardTitle>
            <p class="text-sm text-muted-foreground">
              {{ mode === 'login' ? '登录控制台' : '创建账号' }}
            </p>
          </div>
        </CardHeader>
        <CardContent class="pt-2">
          <form class="space-y-4" @submit.prevent="submit">
            <div class="space-y-2">
              <Label for="username">用户名</Label>
              <Input id="username" v-model="username" autocomplete="username" placeholder="请输入用户名" required />
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
            <template v-if="mode === 'register' && emailRegister">
              <div class="space-y-2">
                <Label>邮箱</Label>
                <Input v-model="email" type="email" placeholder="用于接收验证码" required />
              </div>
              <div class="space-y-2">
                <Label>验证码</Label>
                <div class="flex gap-2">
                  <Input v-model="code" placeholder="邮箱验证码" required />
                  <Button type="button" variant="outline" class="shrink-0 rounded-xl" :disabled="sending" @click="sendCode">
                    {{ sending ? '发送中…' : '获取验证码' }}
                  </Button>
                </div>
              </div>
            </template>
            <p v-if="error" class="rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">{{ error }}</p>
            <Button class="h-11 w-full rounded-xl" :disabled="loading" type="submit">
              <Loader2 v-if="loading" class="size-4 animate-spin" />
              {{ mode === 'login' ? '登录' : '注册' }}
            </Button>

            <template v-if="oauthLogin && mode === 'login'">
              <div class="relative py-1 text-center text-xs text-muted-foreground">
                <span class="relative z-10 bg-background/80 px-2">或</span>
                <div class="absolute inset-x-0 top-1/2 h-px bg-border" />
              </div>
              <Button type="button" variant="outline" class="h-11 w-full rounded-xl" @click="startOAuth">
                {{ oauthLabel }}
              </Button>
            </template>

            <button
              v-if="registerEnabled"
              type="button"
              class="w-full text-center text-sm text-muted-foreground hover:text-foreground"
              @click="mode = mode === 'login' ? 'register' : 'login'"
            >
              {{ mode === 'login' ? '没有账号？注册' : '已有账号？登录' }}
            </button>
          </form>
        </CardContent>
      </Card>
    </div>
  </div>
</template>
