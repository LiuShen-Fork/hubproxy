<script setup lang="ts">
import { onMounted, ref } from 'vue'
import Button from '@/components/ui/Button.vue'
import Input from '@/components/ui/Input.vue'
import Label from '@/components/ui/Label.vue'
import Select from '@/components/ui/Select.vue'
import Card from '@/components/ui/Card.vue'
import CardContent from '@/components/ui/CardContent.vue'
import CardHeader from '@/components/ui/CardHeader.vue'
import CardTitle from '@/components/ui/CardTitle.vue'
import Badge from '@/components/ui/Badge.vue'
import DataTable from '@/components/ui/DataTable.vue'
import { adminApi, formatTime, type User } from '../api'
import { useAuth } from '../auth'
import { toastError, toastSuccess } from '@/lib/toast'
import { pageSlice } from '@/lib/table'

const roleOptions = [
  { value: 'user', label: '普通用户' },
  { value: 'admin', label: '管理员' },
]

const items = ref<User[]>([])
const username = ref('')
const password = ref('')
const role = ref('user')
const tablePage = ref(1)
const tablePageSize = 10
const { user: me } = useAuth()

async function load() {
  const res = await adminApi.users()
  items.value = res.items
}

async function create() {
  try {
    await adminApi.createUser({ username: username.value, password: password.value, role: role.value })
    username.value = ''
    password.value = ''
    toastSuccess('用户已创建')
    await load()
  } catch (e: any) {
    toastError(e?.message || '创建失败')
  }
}

async function remove(id: number) {
  if (!window.confirm('确认删除该用户？')) return
  try {
    await adminApi.deleteUser(id)
    toastSuccess('已删除')
    await load()
  } catch (e: any) {
    toastError(e?.message || '删除失败')
  }
}

async function resetPwd(u: User) {
  const pwd = window.prompt(`为 ${u.username} 设置新密码（至少 8 位）`)
  if (!pwd) return
  try {
    await adminApi.updateUser(u.id, { password: pwd })
    toastSuccess('密码已重置')
  } catch (e: any) {
    toastError(e?.message || '重置失败')
  }
}

async function renameUser(u: User) {
  const name = window.prompt(`修改 ${u.username} 的用户名`, u.username)
  if (!name || name.trim() === u.username) return
  try {
    await adminApi.updateUser(u.id, { username: name.trim() })
    toastSuccess('用户名已更新')
    await load()
  } catch (e: any) {
    toastError(e?.message || '修改失败')
  }
}

async function setLimit(u: User) {
  const raw = window.prompt(
    `设置 ${u.username} 的每日拉取上限（0=不限制，默认 30）`,
    String(u.daily_pull_limit ?? 30),
  )
  if (raw == null) return
  const n = Number(raw)
  if (!Number.isFinite(n) || n < 0 || !Number.isInteger(n)) {
    toastError('请输入非负整数')
    return
  }
  try {
    await adminApi.updateUser(u.id, { daily_pull_limit: n })
    toastSuccess('每日限流已更新')
    await load()
  } catch (e: any) {
    toastError(e?.message || '更新失败')
  }
}

onMounted(load)
</script>

<template>
  <div class="space-y-4">

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
          <Select v-model="role" :options="roleOptions" />
        </div>
        <div class="flex items-end">
          <Button class="w-full rounded-xl" @click="create">创建</Button>
        </div>
      </CardContent>
    </Card>

    <Card>
      <CardContent class="pt-5">
        <DataTable
          v-model:page="tablePage"
          min-width="720px"
          max-height="28rem"
          :paginate="items.length > tablePageSize"
          :total="items.length"
          :page-size="tablePageSize"
        >
          <template #head>
            <tr>
              <th class="px-3 py-2.5 font-medium whitespace-nowrap">ID</th>
              <th class="px-3 py-2.5 font-medium">用户名</th>
              <th class="px-3 py-2.5 font-medium whitespace-nowrap">角色</th>
              <th class="px-3 py-2.5 font-medium whitespace-nowrap">每日拉取上限</th>
              <th class="px-3 py-2.5 font-medium whitespace-nowrap">最近登录</th>
              <th class="px-3 py-2.5 font-medium whitespace-nowrap">操作</th>
            </tr>
          </template>
          <tr
            v-for="u in pageSlice(items, tablePage, tablePageSize)"
            :key="u.id"
            class="border-t border-border/70"
          >
            <td class="px-3 py-2.5 whitespace-nowrap">{{ u.id }}</td>
            <td class="px-3 py-2.5 font-medium">
              <span class="truncate">{{ u.username }}</span>
              <Badge v-if="u.must_change_password" variant="danger" class="ml-2">需改密</Badge>
            </td>
            <td class="px-3 py-2.5 whitespace-nowrap">
              <Badge :variant="u.role === 'admin' ? 'default' : 'secondary'">{{ u.role }}</Badge>
            </td>
            <td class="px-3 py-2.5 tabular-nums whitespace-nowrap">
              {{ u.daily_pull_limit === 0 ? '不限' : u.daily_pull_limit }}
            </td>
            <td class="px-3 py-2.5 whitespace-nowrap">{{ formatTime(u.last_login_at) }}</td>
            <td class="px-3 py-2.5">
              <div class="flex flex-nowrap items-center gap-1">
                <Button size="sm" variant="ghost" @click="setLimit(u)">限流</Button>
                <Button size="sm" variant="ghost" @click="renameUser(u)">改名</Button>
                <Button size="sm" variant="ghost" @click="resetPwd(u)">重置密码</Button>
                <Button size="sm" variant="outline" :disabled="u.id === me?.id" @click="remove(u.id)">删除</Button>
              </div>
            </td>
          </tr>
        </DataTable>
        <p v-if="!items.length" class="py-6 text-center text-sm text-muted-foreground">暂无用户</p>
      </CardContent>
    </Card>
  </div>
</template>
