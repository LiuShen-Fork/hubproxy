<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import Button from '@/components/ui/Button.vue'
import Input from '@/components/ui/Input.vue'
import Select from '@/components/ui/Select.vue'
import DateRange from '@/components/ui/DateRange.vue'
import Card from '@/components/ui/Card.vue'
import CardContent from '@/components/ui/CardContent.vue'
import Badge from '@/components/ui/Badge.vue'
import DataTable from '@/components/ui/DataTable.vue'
import { presetRange, type DateRangePreset } from '@/lib/dateRange'
import { adminApi, formatBytes, type ImageStat } from '../api'

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

async function load() {
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

onMounted(load)
watch(page, load)
</script>

<template>
  <div class="space-y-4">
    <Card>
      <CardContent class="grid gap-3 pt-5 md:grid-cols-4">
        <Input v-model="image" placeholder="镜像名称" />
        <Select v-model="registry" :options="registryOptions" />
        <Select v-model="category" :options="categoryOptions" />
        <Button class="rounded-xl" @click="search">查询</Button>
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
            <td class="px-3 py-2.5 whitespace-nowrap"><Badge variant="secondary">{{ it.category }}</Badge></td>
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
