<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import Button from '@/components/ui/Button.vue'
import Input from '@/components/ui/Input.vue'
import Select from '@/components/ui/Select.vue'
import Card from '@/components/ui/Card.vue'
import CardContent from '@/components/ui/CardContent.vue'
import Badge from '@/components/ui/Badge.vue'
import DataTable from '@/components/ui/DataTable.vue'
import { adminApi, formatBytes, formatTime, type PullSession } from '../api'

const categoryOptions = [
  { value: '', label: '全部类别' },
  { value: 'library', label: 'library' },
  { value: 'user', label: 'user' },
]
const statusOptions = [
  { value: '', label: '全部状态' },
  { value: 'active', label: 'active' },
  { value: 'completed', label: 'completed' },
  { value: 'failed', label: 'failed' },
]

const route = useRoute()
const items = ref<PullSession[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = 20
const ip = ref(String(route.query.ip || ''))
const image = ref('')
const category = ref('')
const status = ref('')
const loading = ref(false)
const selected = ref<{ session: PullSession; events: any[] } | null>(null)

async function load() {
  loading.value = true
  try {
    const res = await adminApi.pulls({
      page: page.value,
      page_size: pageSize,
      ip: ip.value,
      image: image.value,
      category: category.value,
      status: status.value,
    })
    items.value = res.items
    total.value = res.total
  } finally {
    loading.value = false
  }
}

async function openDetail(id: string) {
  selected.value = await adminApi.pull(id)
}

onMounted(load)
watch(page, load)
</script>

<template>
  <div class="space-y-4">
    <Card>
      <CardContent class="grid gap-3 pt-5 md:grid-cols-5">
        <Input v-model="ip" placeholder="按 IP 筛选" />
        <Input v-model="image" placeholder="按镜像名称筛选" />
        <Select v-model="category" :options="categoryOptions" />
        <Select v-model="status" :options="statusOptions" />
        <Button class="rounded-xl" @click="page = 1; load()">查询</Button>
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
              <th class="px-3 py-2.5 font-medium">镜像</th>
              <th class="px-3 py-2.5 font-medium whitespace-nowrap">IP</th>
              <th class="px-3 py-2.5 font-medium whitespace-nowrap">类别</th>
              <th class="px-3 py-2.5 font-medium whitespace-nowrap">层/请求</th>
              <th class="px-3 py-2.5 font-medium whitespace-nowrap">流量</th>
              <th class="px-3 py-2.5 font-medium whitespace-nowrap">状态</th>
              <th class="px-3 py-2.5 font-medium whitespace-nowrap"></th>
            </tr>
          </template>
          <tr v-for="p in items" :key="p.id" class="border-t border-border/70">
            <td class="px-3 py-2.5 whitespace-nowrap">{{ formatTime(p.started_at) }}</td>
            <td class="max-w-[14rem] px-3 py-2.5">
              <div class="truncate font-medium" :title="`${p.image_name}:${p.tag}`">{{ p.image_name }}:{{ p.tag }}</div>
              <div class="truncate text-xs text-muted-foreground">{{ p.registry }}</div>
            </td>
            <td class="px-3 py-2.5 font-mono text-xs whitespace-nowrap">{{ p.client_ip }}</td>
            <td class="px-3 py-2.5 whitespace-nowrap"><Badge variant="secondary">{{ p.category }}</Badge></td>
            <td class="px-3 py-2.5 whitespace-nowrap">{{ p.layer_count }} / {{ p.request_count }}</td>
            <td class="px-3 py-2.5 whitespace-nowrap">{{ formatBytes(p.bytes_total) }}</td>
            <td class="px-3 py-2.5 whitespace-nowrap">
              <Badge :variant="p.status === 'active' ? 'success' : 'outline'">{{ p.status }}</Badge>
            </td>
            <td class="px-3 py-2.5 whitespace-nowrap">
              <Button size="sm" variant="ghost" @click="openDetail(p.id)">详情</Button>
            </td>
          </tr>
        </DataTable>
        <p v-if="!items.length && !loading" class="py-6 text-center text-sm text-muted-foreground">暂无记录</p>
      </CardContent>
    </Card>

    <div v-if="selected" class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4" @click.self="selected = null">
      <Card class="max-h-[80vh] w-full max-w-2xl overflow-auto">
        <CardContent class="space-y-4 pt-5">
          <div class="flex items-start justify-between">
            <div>
              <div class="font-display text-lg font-semibold">{{ selected.session.image_name }}:{{ selected.session.tag }}</div>
              <div class="text-sm text-muted-foreground">{{ selected.session.registry }} · {{ selected.session.client_ip }}</div>
            </div>
            <Button variant="ghost" size="sm" @click="selected = null">关闭</Button>
          </div>
          <div class="grid grid-cols-2 gap-2 text-sm">
            <div>状态：{{ selected.session.status }}</div>
            <div>流量：{{ formatBytes(selected.session.bytes_total) }}</div>
            <div>层数：{{ selected.session.layer_count }}</div>
            <div>HTTP 请求：{{ selected.session.request_count }}</div>
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
