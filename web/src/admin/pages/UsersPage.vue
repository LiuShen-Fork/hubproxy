<script setup lang="ts">
import { onMounted, ref } from 'vue'
import Button from '@/components/ui/Button.vue'
import Input from '@/components/ui/Input.vue'
import Label from '@/components/ui/Label.vue'
import Card from '@/components/ui/Card.vue'
import CardContent from '@/components/ui/CardContent.vue'
import CardHeader from '@/components/ui/CardHeader.vue'
import CardTitle from '@/components/ui/CardTitle.vue'
import Badge from '@/components/ui/Badge.vue'
import { adminApi, formatTime, type User } from '../api'
import { useAuth } from '../auth'

const items = ref<User[]>([])
const username = ref('')
const password = ref('')
const role = ref('user')
const msg = ref('')
const err = ref('')
const { user: me } = useAuth()

async function load() {
  const res = await adminApi.users()
  items.value = res.items
}

async function create() {
  err.value = ''
  try {
    await adminApi.createUser({ username: username.value, password: password.value, role: role.value })
    username.value = ''
    password.value = ''
    msg.value = '用户已创建'
    await load()
  } catch (e: any) {
    err.value = e?.message || '创建失败'
  }
}

async function remove(id: number) {
  if (!confirm('确认删除该用户？')) return
  try {
    await adminApi.deleteUser(id)
    await load()
  } catch (e: any) {
    err.value = e?.message || '删除失败'
  }
}

async function resetPwd(u: User) {
  const pwd = prompt(`为 ${u.username} 设置新密码（至少 8 位）`)
  if (!pwd) return
  try {
    await adminApi.updateUser(u.id, { password: pwd })
    msg.value = '密码已重置'
  } catch (e: any) {
    err.value = e?.message || '重置失败'
  }
}

async function renameUser(u: User) {
  const name = prompt(`修改 ${u.username} 的用户名`, u.username)
  if (!name || name.trim() === u.username) return
  try {
    await adminApi.updateUser(u.id, { username: name.trim() })
    msg.value = '用户名已更新'
    await load()
  } catch (e: any) {
    err.value = e?.message || '修改失败'
  }
}

async function setLimit(u: User) {
  const raw = prompt(
    `设置 ${u.username} 的每日拉取上限（0=不限制，默认 30）`,
    String(u.daily_pull_limit ?? 30),
  )
  if (raw == null) return
  const n = Number(raw)
  if (!Number.isFinite(n) || n < 0 || !Number.isInteger(n)) {
    err.value = '请输入非负整数'
    return
  }
  try {
    await adminApi.updateUser(u.id, { daily_pull_limit: n })
    msg.value = '每日限流已更新'
    await load()
  } catch (e: any) {
    err.value = e?.message || '更新失败'
  }
}

onMounted(load)
</script>

<template>
  <div class="space-y-4">
    <p v-if="msg" class="text-sm text-emerald-600">{{ msg }}</p>
    <p v-if="err" class="text-sm text-destructive">{{ err }}</p>

    <Card>
      <CardHeader>
        <CardTitle>创建用户</CardTitle>
      </CardHeader>
      <CardContent class="grid gap-3 md:grid-cols-4">
        <div class="space-y-2">
          <Label>用户名</Label>
          <Input v-model="username" />
        </div>
        <div class="space-y-2">
          <Label>密码</Label>
          <Input v-model="password" type="password" />
        </div>
        <div class="space-y-2">
          <Label>角色</Label>
          <select v-model="role" class="h-11 w-full rounded-lg border border-input bg-transparent px-3 text-sm">
            <option value="user">user</option>
            <option value="admin">admin</option>
          </select>
        </div>
        <div class="flex items-end">
          <Button class="w-full" @click="create">创建</Button>
        </div>
      </CardContent>
    </Card>

    <Card>
      <CardContent class="overflow-x-auto pt-5">
        <table class="w-full text-sm">
          <thead class="text-left text-muted-foreground">
            <tr>
              <th class="pb-2">ID</th>
              <th class="pb-2">用户名</th>
              <th class="pb-2">角色</th>
              <th class="pb-2">每日拉取上限</th>
              <th class="pb-2">最近登录</th>
              <th class="pb-2"></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="u in items" :key="u.id" class="border-t border-border">
              <td class="py-2">{{ u.id }}</td>
              <td class="py-2 font-medium">
                {{ u.username }}
                <Badge v-if="u.must_change_password" variant="danger" class="ml-2">需改密</Badge>
              </td>
              <td class="py-2"><Badge :variant="u.role === 'admin' ? 'default' : 'secondary'">{{ u.role }}</Badge></td>
              <td class="py-2 tabular-nums">
                {{ u.daily_pull_limit === 0 ? '不限' : u.daily_pull_limit }}
              </td>
              <td class="py-2">{{ formatTime(u.last_login_at) }}</td>
              <td class="py-2 space-x-1">
                <Button size="sm" variant="ghost" @click="setLimit(u)">限流</Button>
                <Button size="sm" variant="ghost" @click="renameUser(u)">改名</Button>
                <Button size="sm" variant="ghost" @click="resetPwd(u)">重置密码</Button>
                <Button size="sm" variant="outline" :disabled="u.id === me?.id" @click="remove(u.id)">删除</Button>
              </td>
            </tr>
          </tbody>
        </table>
      </CardContent>
    </Card>
  </div>
</template>
