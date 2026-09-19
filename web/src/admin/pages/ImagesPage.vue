<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import Button from '@/components/ui/Button.vue'
import Input from '@/components/ui/Input.vue'
import Select from '@/components/ui/Select.vue'
import DateRange from '@/components/ui/DateRange.vue'
import Card from '@/components/ui/Card.vue'
import CardContent from '@/components/ui/CardContent.vue'
import Badge from '@/components/ui/Badge.vue'
import Label from '@/components/ui/Label.vue'
import DataTable from '@/components/ui/DataTable.vue'
import { presetRange, type DateRangePreset } from '@/lib/dateRange'
import { adminApi, categoryLabel, formatBytes, type ImageStat } from '../api'

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

const items = ref<ImageStat[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = 20
const image = ref('')
const category = ref('')
const registry = ref('')
const loading = ref(false)
const dateRange = ref<{ preset: DateRangePreset; from: string; to: string }>({
  preset: 'all',
  from: '',
  to: '',
})

// 请求序号：search() 里 page 的赋值会连带触发 watch，同一次操作可能发出两个请求，
// 且用户连续改筛选条件时新旧请求会并行。只有最后一次发出的请求才有权写入结果。
let seq = 0

async function load() {
  const mine = ++seq
  loading.value = true
  try {
    const range = presetRange(dateRange.value.preset)
    const res = await adminApi.images({
      page: page.value,
      page_size: pageSize,
      image: image.value,
      category: category.value,
      registry: registry.value,
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

onMounted(() => {
  loadRegistries()
  load()
})
watch(page, load)
</script>

<template>
  <div class="space-y-4">
    <Card>
      <CardContent class="grid gap-3 pt-5 md:grid-cols-4">
        <div class="space-y-1.5">
          <Label>镜像名称</Label>
          <Input v-model="image" placeholder="按镜像名筛选" />
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
        <DateRange v-model="dateRange" class="md:col-span-4" @update:model-value="search" />
      </CardContent>
    </Card>
    <Card>
      <CardContent class="pt-5">
        <DataTable
          v-model:page="page"
          min-width="640px"
          max-height="28rem"
          :paginate="total > pageSize"
          :total="total"
          :page-size="pageSize"
        >
          <template #head>
            <tr>
              <th class="px-3 py-2.5 font-medium">镜像</th>
              <th class="px-3 py-2.5 font-medium">Registry</th>
              <th class="px-3 py-2.5 font-medium whitespace-nowrap">类型</th>
              <th class="px-3 py-2.5 font-medium whitespace-nowrap text-right">拉取次数</th>
              <th class="px-3 py-2.5 font-medium whitespace-nowrap text-right">独立 IP</th>
              <th class="px-3 py-2.5 font-medium whitespace-nowrap text-right">总流量</th>
            </tr>
          </template>
          <tr
            v-for="it in items"
            :key="it.registry + it.image_name"
            class="border-t border-border/70 transition-colors hover:bg-accent/40"
          >
            <td class="max-w-[12rem] truncate px-3 py-2.5 font-medium" :title="it.image_name">{{ it.image_name }}</td>
            <td class="max-w-[10rem] truncate px-3 py-2.5" :title="it.registry">{{ it.registry }}</td>
            <td class="px-3 py-2.5 whitespace-nowrap"><Badge variant="secondary">{{ categoryLabel(it.category) }}</Badge></td>
            <td class="px-3 py-2.5 whitespace-nowrap text-right tabular-nums">{{ it.pull_count }}</td>
            <td class="px-3 py-2.5 whitespace-nowrap text-right tabular-nums">{{ it.unique_ips }}</td>
            <td class="px-3 py-2.5 whitespace-nowrap text-right tabular-nums">{{ formatBytes(it.bytes_total) }}</td>
          </tr>
        </DataTable>
        <p v-if="loading" class="py-6 text-center text-sm text-muted-foreground">加载中…</p>
        <p v-else-if="!items.length" class="py-6 text-center text-sm text-muted-foreground">
          没有符合条件的记录
        </p>
      </CardContent>
    </Card>
  </div>
</template>
