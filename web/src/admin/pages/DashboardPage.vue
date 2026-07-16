<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { HardDrive, Network, Package, TrendingUp } from 'lucide-vue-next'
import Card from '@/components/ui/Card.vue'
import CardContent from '@/components/ui/CardContent.vue'
import CardHeader from '@/components/ui/CardHeader.vue'
import CardTitle from '@/components/ui/CardTitle.vue'
import Badge from '@/components/ui/Badge.vue'
import DataTable from '@/components/ui/DataTable.vue'
import { adminApi, formatBytes, formatTime, type DashboardStats } from '../api'
import { pageSlice } from '@/lib/table'

const stats = ref<DashboardStats | null>(null)
const error = ref('')
const recentPage = ref(1)
const recentPageSize = 6
let timer: number | undefined

const recentTotal = computed(() => stats.value?.recent_pulls?.length || 0)
const trend = computed(() => stats.value?.daily_trend || [])
const maxPull = computed(() => Math.max(1, ...trend.value.map((t) => t.pull_count || 0)))

function barHeight(count: number) {
  if (!count) return '0%'
  const h = Math.round((count / maxPull.value) * 100)
  return `${Math.max(h, 5)}%`
}

function tooltipClass(index: number) {
  if (index === 0) return 'left-0 translate-x-0'
  if (index === trend.value.length - 1) return 'right-0 translate-x-0'
  return 'left-1/2 -translate-x-1/2'
}

async function load() {
  try {
    stats.value = await adminApi.dashboard(14)
    error.value = ''
  } catch (e: any) {
    error.value = e?.message || '加载失败'
  }
}

onMounted(() => {
  load()
  timer = window.setInterval(load, 30_000)
})
onUnmounted(() => {
  if (timer) clearInterval(timer)
})
</script>

<template>
  <div class="space-y-6">
    <p v-if="error" class="text-sm text-destructive">{{ error }}</p>

    <div v-if="stats" class="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
      <Card class="min-h-[7.5rem]">
        <CardContent class="flex h-full items-start justify-between pt-5">
          <div>
            <div class="text-sm text-muted-foreground">总拉取次数</div>
            <div class="mt-1 font-display text-3xl font-semibold">{{ stats.total_pulls }}</div>
            <div class="mt-1 text-xs text-muted-foreground">今日 {{ stats.today_pulls }}</div>
          </div>
          <div class="rounded-lg bg-primary/10 p-2 text-primary"><Package class="size-5" /></div>
        </CardContent>
      </Card>
      <Card class="min-h-[7.5rem]">
        <CardContent class="flex h-full items-start justify-between pt-5">
          <div>
            <div class="text-sm text-muted-foreground">总流量</div>
            <div class="mt-1 font-display text-3xl font-semibold">{{ formatBytes(stats.total_bytes) }}</div>
            <div class="mt-1 text-xs text-muted-foreground">今日 {{ formatBytes(stats.today_bytes) }}</div>
          </div>
          <div class="rounded-lg bg-primary/10 p-2 text-primary"><HardDrive class="size-5" /></div>
        </CardContent>
      </Card>
      <Card class="min-h-[7.5rem]">
        <CardContent class="flex h-full items-start justify-between pt-5">
          <div>
            <div class="text-sm text-muted-foreground">独立 IP</div>
            <div class="mt-1 font-display text-3xl font-semibold">{{ stats.unique_ips }}</div>
            <div class="mt-1 text-xs text-muted-foreground">累计去重</div>
          </div>
          <div class="rounded-lg bg-primary/10 p-2 text-primary"><Network class="size-5" /></div>
        </CardContent>
      </Card>
    </div>

    <div v-if="stats" class="grid gap-4 xl:grid-cols-3">
      <Card class="relative flex min-h-[20rem] flex-col xl:col-span-2">
        <CardHeader class="flex-row items-center justify-between space-y-0">
          <CardTitle>近 14 日拉取趋势</CardTitle>
          <TrendingUp class="size-4 text-muted-foreground" />
        </CardHeader>
        <CardContent class="flex flex-1 flex-col overflow-visible">
          <div v-if="trend.some((d) => d.pull_count > 0)" class="flex min-h-[14rem] flex-1 items-end gap-1.5 overflow-visible sm:gap-2">
            <div
              v-for="(d, index) in trend"
              :key="d.day"
              class="group relative flex h-full min-w-0 flex-1 flex-col items-center justify-end overflow-visible hover:z-50"
            >
              <div class="relative flex min-h-0 w-full flex-1 items-end justify-center overflow-visible px-0.5">
                <div
                  class="w-full max-w-[28px] rounded-t-md bg-primary/85 transition-all group-hover:bg-primary"
                  :style="{ height: barHeight(d.pull_count) }"
                />
                <div
                  :class="[
                    'pointer-events-none absolute bottom-full z-50 mb-1 hidden whitespace-nowrap rounded-md bg-foreground px-2 py-1 text-[11px] text-background shadow-lg group-hover:block',
                    tooltipClass(index),
                  ]"
                >
                  {{ d.day }} · {{ d.pull_count }} 次 · {{ formatBytes(d.bytes_total) }}
                </div>
              </div>
              <div class="mt-2 w-full truncate text-center text-[10px] text-muted-foreground">
                {{ d.day.slice(5) }}
              </div>
            </div>
          </div>
          <div v-else class="flex flex-1 items-center justify-center text-sm text-muted-foreground">
            暂无数据
          </div>
        </CardContent>
      </Card>

      <Card class="flex min-h-[20rem] flex-col">
        <CardHeader>
          <CardTitle>镜像类别</CardTitle>
        </CardHeader>
        <CardContent class="flex flex-1 flex-col">
          <div v-if="stats.category_stats?.length" class="space-y-3">
            <div
              v-for="c in stats.category_stats"
              :key="c.category"
              class="flex items-center justify-between gap-2 text-sm"
            >
              <Badge variant="secondary">{{ c.category }}</Badge>
              <span class="shrink-0 text-muted-foreground">{{ c.pull_count }} 次 · {{ formatBytes(c.bytes_total) }}</span>
            </div>
          </div>
          <div v-else class="flex flex-1 items-center justify-center text-sm text-muted-foreground">
            暂无数据
          </div>
        </CardContent>
      </Card>
    </div>

    <div v-if="stats" class="grid gap-4 lg:grid-cols-2">
      <Card class="flex min-h-[18rem] flex-col">
        <CardHeader>
          <CardTitle>热门镜像 Top 10</CardTitle>
        </CardHeader>
        <CardContent class="flex flex-1 flex-col">
          <DataTable v-if="stats.top_images?.length" min-width="420px" max-height="16rem">
            <template #head>
              <tr>
                <th class="px-3 py-2.5 font-medium">镜像</th>
                <th class="px-3 py-2.5 font-medium whitespace-nowrap">次数</th>
                <th class="px-3 py-2.5 font-medium whitespace-nowrap">流量</th>
              </tr>
            </template>
            <tr
              v-for="img in stats.top_images"
              :key="img.registry + img.image_name"
              class="border-t border-border/70"
            >
              <td class="max-w-[14rem] px-3 py-2.5">
                <div class="truncate font-medium" :title="img.image_name">{{ img.image_name }}</div>
                <div class="truncate text-xs text-muted-foreground">{{ img.registry }} · {{ img.category }}</div>
              </td>
              <td class="px-3 py-2.5 whitespace-nowrap">{{ img.pull_count }}</td>
              <td class="px-3 py-2.5 whitespace-nowrap">{{ formatBytes(img.bytes_total) }}</td>
            </tr>
          </DataTable>
          <div v-else class="flex flex-1 items-center justify-center text-sm text-muted-foreground">
            暂无数据
          </div>
        </CardContent>
      </Card>

      <Card class="flex min-h-[18rem] flex-col">
        <CardHeader>
          <CardTitle>活跃 IP Top 10</CardTitle>
        </CardHeader>
        <CardContent class="flex flex-1 flex-col">
          <DataTable v-if="stats.top_ips?.length" min-width="360px" max-height="16rem">
            <template #head>
              <tr>
                <th class="px-3 py-2.5 font-medium">IP</th>
                <th class="px-3 py-2.5 font-medium whitespace-nowrap">次数</th>
                <th class="px-3 py-2.5 font-medium whitespace-nowrap">流量</th>
              </tr>
            </template>
            <tr v-for="ip in stats.top_ips" :key="ip.client_ip" class="border-t border-border/70">
              <td class="px-3 py-2.5 font-mono text-xs whitespace-nowrap">{{ ip.client_ip }}</td>
              <td class="px-3 py-2.5 whitespace-nowrap">{{ ip.pull_count }}</td>
              <td class="px-3 py-2.5 whitespace-nowrap">{{ formatBytes(ip.bytes_total) }}</td>
            </tr>
          </DataTable>
          <div v-else class="flex flex-1 items-center justify-center text-sm text-muted-foreground">
            暂无数据
          </div>
        </CardContent>
      </Card>
    </div>

    <Card v-if="stats" class="flex min-h-[18rem] flex-col">
      <CardHeader>
        <CardTitle>最近拉取</CardTitle>
      </CardHeader>
      <CardContent class="flex flex-1 flex-col">
        <DataTable
          v-if="recentTotal"
          v-model:page="recentPage"
          min-width="520px"
          max-height="20rem"
          :paginate="recentTotal > recentPageSize"
          :total="recentTotal"
          :page-size="recentPageSize"
        >
          <template #head>
            <tr>
              <th class="px-3 py-2.5 font-medium whitespace-nowrap">时间</th>
              <th class="px-3 py-2.5 font-medium">镜像</th>
              <th class="px-3 py-2.5 font-medium whitespace-nowrap">IP</th>
              <th class="px-3 py-2.5 font-medium whitespace-nowrap">流量</th>
            </tr>
          </template>
          <tr
            v-for="p in pageSlice(stats.recent_pulls, recentPage, recentPageSize)"
            :key="p.id"
            class="border-t border-border/70"
          >
            <td class="px-3 py-2.5 whitespace-nowrap">{{ formatTime(p.started_at) }}</td>
            <td class="max-w-[12rem] px-3 py-2.5">
              <div class="truncate" :title="`${p.image_name}:${p.tag}`">{{ p.image_name }}:{{ p.tag }}</div>
              <div class="truncate text-xs text-muted-foreground">{{ p.registry }}</div>
            </td>
            <td class="px-3 py-2.5 font-mono text-xs whitespace-nowrap">{{ p.client_ip }}</td>
            <td class="px-3 py-2.5 whitespace-nowrap">{{ formatBytes(p.bytes_total) }}</td>
          </tr>
        </DataTable>
        <div v-else class="flex flex-1 items-center justify-center text-sm text-muted-foreground">
          暂无数据
        </div>
      </CardContent>
    </Card>
  </div>
</template>
