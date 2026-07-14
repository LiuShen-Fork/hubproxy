<script setup lang="ts">
import { ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import Button from '@/components/ui/Button.vue'
import Input from '@/components/ui/Input.vue'
import Label from '@/components/ui/Label.vue'
import Card from '@/components/ui/Card.vue'
import CardContent from '@/components/ui/CardContent.vue'
import CardHeader from '@/components/ui/CardHeader.vue'
import CardTitle from '@/components/ui/CardTitle.vue'
import { adminApi, setToken } from '../api'
import { useAuth } from '../auth'

const router = useRouter()
const { user, setUser } = useAuth()
const username = ref(user.value?.username || '')
const current = ref('')
const next = ref('')
const confirm = ref('')
const error = ref('')
const msg = ref('')
const loading = ref(false)

watch(
  () => user.value?.username,
  (v) => {
    if (v) username.value = v
  },
)

async function submit() {
  error.value = ''
  msg.value = ''
  const newName = username.value.trim()
  const changingUsername = !!newName && newName.toLowerCase() !== (user.value?.username || '').toLowerCase()
  const changingPassword = !!next.value

  if (!newName || newName.length < 2) {
    error.value = '用户名至少 2 位'
    return
  }
  if (!/^[A-Za-z0-9_-]+$/.test(newName)) {
    error.value = '用户名仅允许字母、数字、下划线和连字符'
    return
  }
  if (!changingUsername && !changingPassword) {
    error.value = '请修改用户名或密码后再保存'
    return
  }
  if (user.value?.must_change_password && !changingPassword) {
    error.value = '首次登录必须修改密码'
    return
  }
  if (changingPassword) {
    if (next.value.length < 8) {
      error.value = '新密码至少 8 位'
      return
    }
    if (next.value !== confirm.value) {
      error.value = '两次输入的新密码不一致'
      return
    }
  }
  if (!user.value?.must_change_password && !current.value) {
    error.value = '请填写当前密码以验证身份'
    return
  }

  loading.value = true
  try {
    // 始终提交 username，后端忽略未变更的情况；避免只改名时字段丢失
    const res = await adminApi.updateProfile({
      username: newName,
      current_password: current.value || undefined,
      new_password: changingPassword ? next.value : undefined,
    })
    if (res.token) setToken(res.token)
    if (res.user) setUser(res.user)
    msg.value = '资料已更新'
    current.value = ''
    next.value = ''
    confirm.value = ''
    if (res.user && !res.user.must_change_password) {
      await router.replace(res.user.role === 'admin' ? '/admin' : '/admin/user')
    }
  } catch (e: any) {
    error.value = e?.message || '修改失败'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="mx-auto max-w-lg">
    <Card>
      <CardHeader>
        <CardTitle>账户资料</CardTitle>
        <p class="text-sm text-muted-foreground">
          <span v-if="user?.must_change_password">首次登录请修改默认密码，可同时改用户名（无需填当前密码）。</span>
          <span v-else>可修改用户名与密码；提交时需填写当前密码，改密后其他会话失效。</span>
        </p>
      </CardHeader>
      <CardContent>
        <form class="space-y-4" @submit.prevent="submit">
          <div class="space-y-2">
            <Label>用户名</Label>
            <Input v-model="username" autocomplete="username" maxlength="32" />
            <p class="text-xs text-muted-foreground">2-32 位，仅字母、数字、下划线与连字符（如 LiuShen）</p>
          </div>
          <div class="space-y-2">
            <Label>当前密码</Label>
            <Input
              v-model="current"
              type="password"
              autocomplete="current-password"
              :placeholder="user?.must_change_password ? '首次改密可不填' : '必填，用于验证身份'"
            />
          </div>
          <div class="space-y-2">
            <Label>新密码</Label>
            <Input
              v-model="next"
              type="password"
              autocomplete="new-password"
              :placeholder="user?.must_change_password ? '必填，至少 8 位' : '不修改请留空'"
            />
          </div>
          <div class="space-y-2">
            <Label>确认新密码</Label>
            <Input
              v-model="confirm"
              type="password"
              autocomplete="new-password"
              :placeholder="next ? '再次输入新密码' : '不修改请留空'"
            />
          </div>
          <p v-if="error" class="text-sm text-destructive">{{ error }}</p>
          <p v-if="msg" class="text-sm text-emerald-600">{{ msg }}</p>
          <Button type="submit" :disabled="loading">保存</Button>
        </form>
      </CardContent>
    </Card>
  </div>
</template>
