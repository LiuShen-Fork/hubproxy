<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { Gauge, HardDrive, Network, Package } from 'lucide-vue-next'
import Card from '@/components/ui/Card.vue'
import CardContent from '@/components/ui/CardContent.vue'
import CardHeader from '@/components/ui/CardHeader.vue'
import CardTitle from '@/components/ui/CardTitle.vue'
import DataTable from '@/components/ui/DataTable.vue'
import { adminApi, formatBytes, formatTime, type DashboardStats, type UserQuota } from '../api'
import { pageSlice } from '@/lib/table'

const stats = ref<DashboardStats | null>(null)
const quota = ref<UserQuota | null>(null)
const error = ref('')
const recentPage = ref(1)
const recentPageSize = 6
let timer: number | undefined

const recentTotal = computed(() => stats.value?.recent_pulls?.length || 0)

async function load() {
  try {
    const res = await adminApi.userDashboard(14)
    stats.value = res.stats
    quota.value = res.quota
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

    <div v-if="quota" class="rounded-xl border border-primary/20 bg-primary/5 p-5">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div class="flex items-center gap-3">
          <div class="rounded-lg bg-primary/15 p-2 text-primary"><Gauge class="size-5" /></div>
          <div>
            <div class="font-display text-lg font-semibold">今日拉取配额</div>
            <div class="text-sm text-muted-foreground">
              每日本地时间 0 点刷新 · 下次重置 {{ quota.resets_at_human }}
            </div>
          </div>
        </div>
        <div class="text-right">
          <div class="font-display text-3xl font-semibold tabular-nums">
            <span :class="quota.remaining <= 3 ? 'text-destructive' : ''">{{ quota.remaining }}</span>
            <span class="text-lg text-muted-foreground"> / {{ quota.daily_limit === 0 ? '∞' : quota.daily_limit }}</span>
          </div>
          <div class="text-xs text-muted-foreground">剩余 / 上限（今日已用 {{ quota.used_today }}）</div>
        </div>
      </div>
      <div v-if="quota.daily_limit > 0" class="mt-3 h-2 overflow-hidden rounded-full bg-muted">
        <div
          class="h-full rounded-full bg-primary transition-all"
          :style="{ width: `${Math.min(100, (quota.used_today / quota.daily_limit) * 100)}%` }"
        />
      </div>
    </div>

    <div v-if="stats" class="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
      <Card class="min-h-[7.5rem]">
        <CardContent class="flex h-full items-start justify-between pt-5">
          <div>
            <div class="text-sm text-muted-foreground">我的总拉取</div>
            <div class="mt-1 font-display text-3xl font-semibold">{{ stats.total_pulls }}</div>
            <div class="mt-1 text-xs text-muted-foreground">今日 {{ stats.today_pulls }}</div>
          </div>
          <div class="rounded-lg bg-primary/10 p-2 text-primary"><Package class="size-5" /></div>
        </CardContent>
      </Card>
      <Card class="min-h-[7.5rem]">
        <CardContent class="flex h-full items-start justify-between pt-5">
          <div>
            <div class="text-sm text-muted-foreground">我的流量</div>
            <div class="mt-1 font-display text-3xl font-semibold">{{ formatBytes(stats.total_bytes) }}</div>
            <div class="mt-1 text-xs text-muted-foreground">今日 {{ formatBytes(stats.today_bytes) }}</div>
          </div>
          <div class="rounded-lg bg-primary/10 p-2 text-primary"><HardDrive class="size-5" /></div>
        </CardContent>
      </Card>
      <Card class="min-h-[7.5rem]">
        <CardContent class="flex h-full items-start justify-between pt-5">
          <div>
            <div class="text-sm text-muted-foreground">使用 IP 数</div>
            <div class="mt-1 font-display text-3xl font-semibold">{{ stats.unique_ips }}</div>
            <div class="mt-1 text-xs text-muted-foreground">累计去重</div>
          </div>
          <div class="rounded-lg bg-primary/10 p-2 text-primary"><Network class="size-5" /></div>
        </CardContent>
      </Card>
    </div>

    <Card v-if="stats" class="flex min-h-[18rem] flex-col">
      <CardHeader>
        <CardTitle>最近拉取（本账号令牌）</CardTitle>
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
            </td>
            <td class="px-3 py-2.5 font-mono text-xs whitespace-nowrap">{{ p.client_ip }}</td>
            <td class="px-3 py-2.5 whitespace-nowrap">{{ formatBytes(p.bytes_total) }}</td>
          </tr>
        </DataTable>
        <div v-else class="flex flex-1 items-center justify-center text-sm text-muted-foreground">
          暂无拉取记录
        </div>
      </CardContent>
    </Card>
  </div>
</template>
