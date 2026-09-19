<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import Button from '@/components/ui/Button.vue'
import Input from '@/components/ui/Input.vue'
import Select from '@/components/ui/Select.vue'
import MultiSelect from '@/components/ui/MultiSelect.vue'
import Autocomplete from '@/components/ui/Autocomplete.vue'
import DateRange from '@/components/ui/DateRange.vue'
import Card from '@/components/ui/Card.vue'
import CardContent from '@/components/ui/CardContent.vue'
import Badge from '@/components/ui/Badge.vue'
import DataTable from '@/components/ui/DataTable.vue'
import { presetRange, type DateRangePreset } from '@/lib/dateRange'
import {
  adminApi,
  displayPullName,
  formatBytes,
  formatTime,
  type PullSession,
  type User,
} from '../api'

// 来源与类型是两个不同维度：registry 是仓库来源，category 是镜像归属
const registryOptions = [
  { value: '', label: '全部来源' },
  { value: 'docker.io', label: 'docker.io' },
  { value: 'ghcr.io', label: 'ghcr.io' },
  { value: 'gcr.io', label: 'gcr.io' },
  { value: 'quay.io', label: 'quay.io' },
  { value: 'registry.k8s.io', label: 'registry.k8s.io' },
  { value: 'registry.gitlab.com', label: 'registry.gitlab.com' },
]
const categoryOptions = [
  { value: '', label: '全部类型' },
  { value: 'library', label: 'library' },
  { value: 'user', label: 'user' },
]

const route = useRoute()
const items = ref<PullSession[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = 20
const loading = ref(false)
const selected = ref<{ session: PullSession; events: any[] } | null>(null)

const ip = ref(String(route.query.ip || ''))
const image = ref('')
const registry = ref('')
const category = ref('')
const userIds = ref<string[]>([])
const userOptions = ref<{ value: string; label: string }[]>([])
const dateRange = ref<{ preset: DateRangePreset; from: string; to: string }>({
  preset: 'all',
  from: '',
  to: '',
})

async function loadUsers() {
  try {
    const res = await adminApi.users()
    userOptions.value = res.items.map((u: User) => ({
      value: String(u.id),
      label: u.username,
    }))
  } catch {
    // 拉不到用户列表时筛选框为空，表格仍可用（只是没有名字可显示）
  }
}

async function load() {
  loading.value = true
  try {
    const range = presetRange(dateRange.value.preset)
    const res = await adminApi.pulls({
      page: page.value,
      page_size: pageSize,
      ip: ip.value,
      image: image.value,
      registry: registry.value,
      category: category.value,
      user_id: userIds.value,
      from: dateRange.value.preset === 'custom' ? dateRange.value.from : range.from,
      to: dateRange.value.preset === 'custom' ? dateRange.value.to : range.to,
    })
    items.value = res.items
    total.value = res.total
  } finally {
    loading.value = false
  }
}

function search() {
  page.value = 1
  load()
}

async function openDetail(id: string) {
  selected.value = await adminApi.pull(id)
}

onMounted(() => {
  loadUsers()
  load()
})
watch(page, load)
</script>

<template>
  <div class="space-y-4">
    <Card>
      <CardContent class="grid gap-3 pt-5 md:grid-cols-3">
        <Autocomplete
          v-model="ip"
          :fetch="(q) => adminApi.ipsSuggest(q).then((r) => r.items)"
          placeholder="按 IP 筛选（输入前几位会有建议）"
        />
        <Input v-model="image" placeholder="按镜像名称筛选" />
        <MultiSelect v-model="userIds" :options="userOptions" placeholder="全部用户" />
        <Select v-model="registry" :options="registryOptions" />
        <Select v-model="category" :options="categoryOptions" />
        <Button class="rounded-xl" @click="search">查询</Button>
        <DateRange v-model="dateRange" class="md:col-span-3" @update:model-value="search" />
      </CardContent>
    </Card>

    <Card>
      <CardContent class="pt-5">
        <DataTable
          v-model:page="page"
          min-width="720px"
          max-height="28rem"
          :paginate="total > pageSize"
          :total="total"
          :page-size="pageSize"
        >
          <template #head>
            <tr>
              <th class="px-3 py-2.5 font-medium whitespace-nowrap">开始时间</th>
              <th class="px-3 py-2.5 font-medium">内容</th>
              <th class="px-3 py-2.5 font-medium whitespace-nowrap">用户</th>
              <th class="px-3 py-2.5 font-medium whitespace-nowrap">IP</th>
              <th class="px-3 py-2.5 font-medium whitespace-nowrap">类型</th>
              <th class="px-3 py-2.5 font-medium whitespace-nowrap text-right">流量</th>
              <th class="px-3 py-2.5 font-medium whitespace-nowrap"></th>
            </tr>
          </template>
          <tr
            v-for="p in items"
            :key="p.id"
            class="border-t border-border/70 transition-colors hover:bg-accent/40"
          >
            <td class="px-3 py-2.5 tabular-nums whitespace-nowrap">{{ formatTime(p.started_at) }}</td>
            <td class="max-w-[14rem] px-3 py-2.5">
              <div class="truncate font-medium" :title="p.image_name">{{ displayPullName(p) }}</div>
              <div class="truncate text-xs text-muted-foreground">{{ p.registry }}</div>
            </td>
            <td class="px-3 py-2.5 whitespace-nowrap">
              <span v-if="p.username">{{ p.username }}</span>
              <span v-else-if="p.user_id" class="text-muted-foreground">#{{ p.user_id }}</span>
              <span v-else class="text-muted-foreground">匿名</span>
            </td>
            <td class="px-3 py-2.5 font-mono text-xs whitespace-nowrap">{{ p.client_ip }}</td>
            <td class="px-3 py-2.5 whitespace-nowrap"><Badge variant="secondary">{{ p.category }}</Badge></td>
            <td class="px-3 py-2.5 text-right tabular-nums whitespace-nowrap">{{ formatBytes(p.bytes_total) }}</td>
            <td class="px-3 py-2.5 whitespace-nowrap">
              <Button size="sm" variant="ghost" @click="openDetail(p.id)">详情</Button>
            </td>
          </tr>
        </DataTable>
        <p v-if="loading" class="py-6 text-center text-sm text-muted-foreground">加载中…</p>
        <p v-else-if="!items.length" class="py-6 text-center text-sm text-muted-foreground">
          没有符合条件的记录，试试放宽筛选条件
        </p>
      </CardContent>
    </Card>

    <div v-if="selected" class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4" @click.self="selected = null">
      <Card class="max-h-[80vh] w-full max-w-2xl overflow-auto">
        <CardContent class="space-y-4 pt-5">
          <div class="flex items-start justify-between">
            <div>
              <div class="font-display text-lg font-semibold">{{ displayPullName(selected.session) }}</div>
              <div class="text-sm text-muted-foreground">{{ selected.session.registry }} · {{ selected.session.client_ip }}</div>
            </div>
            <Button variant="ghost" size="sm" @click="selected = null">关闭</Button>
          </div>
          <div class="grid grid-cols-2 gap-2 text-sm">
            <div>流量：{{ formatBytes(selected.session.bytes_total) }}</div>
            <div>类别：<Badge variant="secondary">{{ selected.session.category }}</Badge></div>
            <div>
              用户：{{ selected.session.username || (selected.session.user_id ? '#' + selected.session.user_id : '匿名') }}
            </div>
            <div>IP：{{ selected.session.client_ip }}</div>
          </div>
          <div>
            <div class="mb-2 text-sm font-medium">事件明细（分片聚合到同一次拉取）</div>
            <div class="max-h-64 space-y-1 overflow-auto text-xs">
              <div v-for="e in selected.events" :key="e.id" class="rounded border border-border px-2 py-1 font-mono">
                {{ formatTime(e.created_at) }} · {{ e.event_type }} · {{ formatBytes(e.bytes) }}
                <span v-if="e.reference" class="text-muted-foreground"> · {{ e.reference.slice(0, 24) }}</span>
              </div>
              <div v-if="!selected.events.length" class="text-muted-foreground">无事件</div>
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  </div>
</template>
