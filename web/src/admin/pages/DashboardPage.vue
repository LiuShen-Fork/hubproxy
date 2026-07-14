<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { Activity, HardDrive, Network, Package } from 'lucide-vue-next'
import Card from '@/components/ui/Card.vue'
import CardContent from '@/components/ui/CardContent.vue'
import CardHeader from '@/components/ui/CardHeader.vue'
import CardTitle from '@/components/ui/CardTitle.vue'
import Badge from '@/components/ui/Badge.vue'
import { adminApi, formatBytes, formatTime, type DashboardStats } from '../api'

const stats = ref<DashboardStats | null>(null)
const error = ref('')
let timer: number | undefined

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

function maxPull(trend: DashboardStats['daily_trend']) {
  return Math.max(1, ...trend.map((t) => t.pull_count))
}
</script>

<template>
  <div class="space-y-6">
    <p v-if="error" class="text-sm text-destructive">{{ error }}</p>

    <div v-if="stats" class="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
      <Card>
        <CardContent class="flex items-start justify-between pt-5">
          <div>
            <div class="text-sm text-muted-foreground">总拉取次数</div>
            <div class="mt-1 font-display text-3xl font-semibold">{{ stats.total_pulls }}</div>
            <div class="mt-1 text-xs text-muted-foreground">今日 {{ stats.today_pulls }}</div>
          </div>
          <div class="rounded-lg bg-primary/10 p-2 text-primary"><Package class="size-5" /></div>
        </CardContent>
      </Card>
      <Card>
        <CardContent class="flex items-start justify-between pt-5">
          <div>
            <div class="text-sm text-muted-foreground">总流量</div>
            <div class="mt-1 font-display text-3xl font-semibold">{{ formatBytes(stats.total_bytes) }}</div>
            <div class="mt-1 text-xs text-muted-foreground">今日 {{ formatBytes(stats.today_bytes) }}</div>
          </div>
          <div class="rounded-lg bg-primary/10 p-2 text-primary"><HardDrive class="size-5" /></div>
        </CardContent>
      </Card>
      <Card>
        <CardContent class="flex items-start justify-between pt-5">
          <div>
            <div class="text-sm text-muted-foreground">独立 IP</div>
            <div class="mt-1 font-display text-3xl font-semibold">{{ stats.unique_ips }}</div>
          </div>
          <div class="rounded-lg bg-primary/10 p-2 text-primary"><Network class="size-5" /></div>
        </CardContent>
      </Card>
      <Card>
        <CardContent class="flex items-start justify-between pt-5">
          <div>
            <div class="text-sm text-muted-foreground">进行中拉取</div>
            <div class="mt-1 font-display text-3xl font-semibold">{{ stats.active_pulls }}</div>
          </div>
          <div class="rounded-lg bg-primary/10 p-2 text-primary"><Activity class="size-5" /></div>
        </CardContent>
      </Card>
    </div>

    <div v-if="stats" class="grid gap-4 xl:grid-cols-3">
      <Card class="xl:col-span-2">
        <CardHeader>
          <CardTitle>近 14 日拉取趋势</CardTitle>
        </CardHeader>
        <CardContent>
          <div class="flex h-48 items-end gap-1.5">
            <div
              v-for="d in stats.daily_trend"
              :key="d.day"
              class="group relative flex flex-1 flex-col items-center justify-end"
            >
              <div
                class="w-full rounded-t-md bg-primary/80 transition-all group-hover:bg-primary"
                :style="{ height: `${(d.pull_count / maxPull(stats.daily_trend)) * 100}%`, minHeight: d.pull_count ? '4px' : '0' }"
              />
              <div class="mt-2 truncate text-[10px] text-muted-foreground">{{ d.day.slice(5) }}</div>
              <div class="pointer-events-none absolute -top-8 hidden rounded bg-foreground px-2 py-1 text-xs text-background group-hover:block">
                {{ d.pull_count }} · {{ formatBytes(d.bytes_total) }}
              </div>
            </div>
            <div v-if="!stats.daily_trend.length" class="w-full py-16 text-center text-sm text-muted-foreground">
              暂无数据
            </div>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>镜像类别</CardTitle>
        </CardHeader>
        <CardContent class="space-y-3">
          <div v-for="c in stats.category_stats" :key="c.category" class="flex items-center justify-between text-sm">
            <Badge variant="secondary">{{ c.category }}</Badge>
            <span>{{ c.pull_count }} 次 · {{ formatBytes(c.bytes_total) }}</span>
          </div>
          <div v-if="!stats.category_stats.length" class="text-sm text-muted-foreground">暂无数据</div>
        </CardContent>
      </Card>
    </div>

    <div v-if="stats" class="grid gap-4 lg:grid-cols-2">
      <Card>
        <CardHeader>
          <CardTitle>热门镜像 Top 10</CardTitle>
        </CardHeader>
        <CardContent class="overflow-x-auto">
          <table class="w-full text-sm">
            <thead class="text-left text-muted-foreground">
              <tr>
                <th class="pb-2 font-medium">镜像</th>
                <th class="pb-2 font-medium">次数</th>
                <th class="pb-2 font-medium">流量</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="img in stats.top_images" :key="img.registry + img.image_name" class="border-t border-border">
                <td class="py-2">
                  <div class="font-medium">{{ img.image_name }}</div>
                  <div class="text-xs text-muted-foreground">{{ img.registry }} · {{ img.category }}</div>
                </td>
                <td class="py-2">{{ img.pull_count }}</td>
                <td class="py-2">{{ formatBytes(img.bytes_total) }}</td>
              </tr>
            </tbody>
          </table>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>活跃 IP Top 10</CardTitle>
        </CardHeader>
        <CardContent class="overflow-x-auto">
          <table class="w-full text-sm">
            <thead class="text-left text-muted-foreground">
              <tr>
                <th class="pb-2 font-medium">IP</th>
                <th class="pb-2 font-medium">次数</th>
                <th class="pb-2 font-medium">流量</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="ip in stats.top_ips" :key="ip.client_ip" class="border-t border-border">
                <td class="py-2 font-mono text-xs">{{ ip.client_ip }}</td>
                <td class="py-2">{{ ip.pull_count }}</td>
                <td class="py-2">{{ formatBytes(ip.bytes_total) }}</td>
              </tr>
            </tbody>
          </table>
        </CardContent>
      </Card>
    </div>

    <Card v-if="stats">
      <CardHeader>
        <CardTitle>最近拉取</CardTitle>
      </CardHeader>
      <CardContent class="overflow-x-auto">
        <table class="w-full text-sm">
          <thead class="text-left text-muted-foreground">
            <tr>
              <th class="pb-2 font-medium">时间</th>
              <th class="pb-2 font-medium">镜像</th>
              <th class="pb-2 font-medium">IP</th>
              <th class="pb-2 font-medium">流量</th>
              <th class="pb-2 font-medium">状态</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="p in stats.recent_pulls" :key="p.id" class="border-t border-border">
              <td class="py-2 whitespace-nowrap">{{ formatTime(p.started_at) }}</td>
              <td class="py-2">
                <div>{{ p.image_name }}:{{ p.tag }}</div>
                <div class="text-xs text-muted-foreground">{{ p.registry }}</div>
              </td>
              <td class="py-2 font-mono text-xs">{{ p.client_ip }}</td>
              <td class="py-2">{{ formatBytes(p.bytes_total) }}</td>
              <td class="py-2">
                <Badge :variant="p.status === 'active' ? 'success' : 'secondary'">{{ p.status }}</Badge>
              </td>
            </tr>
          </tbody>
        </table>
      </CardContent>
    </Card>
  </div>
</template>
