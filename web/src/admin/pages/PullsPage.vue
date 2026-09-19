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
import Label from '@/components/ui/Label.vue'
import DataTable from '@/components/ui/DataTable.vue'
import { presetRange, type DateRangePreset } from '@/lib/dateRange'
import {
  adminApi,
  categoryLabel,
  displayPullName,
  formatBytes,
  formatTime,
  type PullSession,
  type User,
} from '../api'

// 来源与类型是两个不同维度：registry 是仓库来源，category 是镜像归属。
// 类型固定只有 library/user 两种（见后端 ImageCategory），所以写死；
// 来源是动态的：配置可在运行时改，且历史行可能属于配置里已移除的来源，因此取自数据。
const registryOptions = ref<{ value: string; label: string }[]>([
  { value: '', label: '全部来源' },
])
const categoryOptions = [
  { value: '', label: '全部类型' },
  { value: 'library', label: '官方库镜像' },
  { value: 'user', label: '用户镜像' },
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

async function loadRegistries() {
  try {
    const res = await adminApi.registries()
    registryOptions.value = [
      { value: '', label: '全部来源' },
      ...res.items.map((r: string) => ({ value: r, label: r })),
    ]
  } catch {
    // 拉不到来源列表时只剩「全部来源」，表格仍可用（只是筛不了来源）
  }
}

// 请求序号：search() 里 page 的赋值会连带触发 watch，同一次操作可能发出两个请求，
// 且用户连续改筛选条件时新旧请求会并行。只有最后一次发出的请求才有权写入结果。
let seq = 0

async function load() {
  const mine = ++seq
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
    // 丢弃过期响应，避免慢的旧筛选结果覆盖新筛选结果
    if (mine !== seq) return
    items.value = res.items
    total.value = res.total
  } finally {
    // 同样只有最新请求能关掉加载态，否则旧响应会把新请求的「加载中」提前抹掉
    if (mine === seq) loading.value = false
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
  loadRegistries()
  load()
})
watch(page, load)
</script>

<template>
  <div class="space-y-4">
    <Card>
      <CardContent class="grid gap-3 pt-5 md:grid-cols-3">
        <div class="space-y-1.5">
          <Label>IP</Label>
          <Autocomplete
            v-model="ip"
            :fetch="(q) => adminApi.ipsSuggest(q).then((r) => r.items)"
            placeholder="输入前几位会有建议"
          />
        </div>
        <div class="space-y-1.5">
          <Label>镜像名称</Label>
          <Input v-model="image" placeholder="按镜像名筛选" />
        </div>
        <div class="space-y-1.5">
          <Label>用户</Label>
          <MultiSelect v-model="userIds" :options="userOptions" placeholder="全部用户" />
        </div>
        <div class="space-y-1.5">
          <Label>来源（registry）</Label>
          <Select v-model="registry" :options="registryOptions" />
        </div>
        <div class="space-y-1.5">
          <Label>类型</Label>
          <Select v-model="category" :options="categoryOptions" />
        </div>
        <div class="flex items-end">
          <Button class="rounded-xl" @click="search">查询</Button>
        </div>
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
              <div class="truncate text-xs text-muted-foreground">来源：{{ p.registry }}</div>
            </td>
            <td class="px-3 py-2.5 whitespace-nowrap">
              <span v-if="p.username">{{ p.username }}</span>
              <span v-else-if="p.user_id" class="text-muted-foreground">#{{ p.user_id }}</span>
              <span v-else class="text-muted-foreground">匿名</span>
            </td>
            <td class="px-3 py-2.5 font-mono text-xs whitespace-nowrap">{{ p.client_ip }}</td>
            <td class="px-3 py-2.5 whitespace-nowrap"><Badge variant="secondary">{{ categoryLabel(p.category) }}</Badge></td>
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
