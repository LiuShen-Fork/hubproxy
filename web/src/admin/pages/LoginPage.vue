<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { Container, Loader2 } from 'lucide-vue-next'
import Button from '@/components/ui/Button.vue'
import Input from '@/components/ui/Input.vue'
import Label from '@/components/ui/Label.vue'
import Card from '@/components/ui/Card.vue'
import CardContent from '@/components/ui/CardContent.vue'
import CardHeader from '@/components/ui/CardHeader.vue'
import CardTitle from '@/components/ui/CardTitle.vue'
import { adminApi } from '../api'
import { useAuth } from '../auth'

const router = useRouter()
const { login } = useAuth()
const username = ref('admin')
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
      await router.replace('/admin')
    }
  } catch (e: any) {
    error.value = e?.message || '登录失败'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="flex min-h-screen items-center justify-center bg-muted/50 p-4">
    <Card class="w-full max-w-md shadow-lg">
      <CardHeader class="items-center text-center">
        <div class="mb-2 flex size-12 items-center justify-center rounded-xl bg-primary text-primary-foreground">
          <Container class="size-6" />
        </div>
        <CardTitle class="font-display text-xl">清羽镜像 · 管理后台</CardTitle>
        <p class="text-sm text-muted-foreground">登录后管理限流、黑白名单与拉取统计</p>
      </CardHeader>
      <CardContent>
        <form class="space-y-4" @submit.prevent="submit">
          <div class="space-y-2">
            <Label for="username">用户名</Label>
            <Input id="username" v-model="username" autocomplete="username" required />
          </div>
          <div class="space-y-2">
            <Label for="password">密码</Label>
            <Input
              id="password"
              v-model="password"
              type="password"
              autocomplete="current-password"
              required
            />
          </div>
          <p v-if="error" class="text-sm text-destructive">{{ error }}</p>
          <Button class="w-full" :disabled="loading" type="submit">
            <Loader2 v-if="loading" class="size-4 animate-spin" />
            {{ mode === 'login' ? '登录' : '注册' }}
          </Button>
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
</template>
