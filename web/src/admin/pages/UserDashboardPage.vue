<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { Activity, Gauge, HardDrive, Network, Package } from 'lucide-vue-next'
import Card from '@/components/ui/Card.vue'
import CardContent from '@/components/ui/CardContent.vue'
import CardHeader from '@/components/ui/CardHeader.vue'
import CardTitle from '@/components/ui/CardTitle.vue'
import Badge from '@/components/ui/Badge.vue'
import { adminApi, formatBytes, formatTime, type DashboardStats, type UserQuota } from '../api'

const stats = ref<DashboardStats | null>(null)
const quota = ref<UserQuota | null>(null)
const error = ref('')
let timer: number | undefined

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

    <div v-if="stats" class="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
      <Card>
        <CardContent class="flex items-start justify-between pt-5">
          <div>
            <div class="text-sm text-muted-foreground">我的总拉取</div>
            <div class="mt-1 font-display text-3xl font-semibold">{{ stats.total_pulls }}</div>
            <div class="mt-1 text-xs text-muted-foreground">今日 {{ stats.today_pulls }}</div>
          </div>
          <div class="rounded-lg bg-primary/10 p-2 text-primary"><Package class="size-5" /></div>
        </CardContent>
      </Card>
      <Card>
        <CardContent class="flex items-start justify-between pt-5">
          <div>
            <div class="text-sm text-muted-foreground">我的流量</div>
            <div class="mt-1 font-display text-3xl font-semibold">{{ formatBytes(stats.total_bytes) }}</div>
            <div class="mt-1 text-xs text-muted-foreground">今日 {{ formatBytes(stats.today_bytes) }}</div>
          </div>
          <div class="rounded-lg bg-primary/10 p-2 text-primary"><HardDrive class="size-5" /></div>
        </CardContent>
      </Card>
      <Card>
        <CardContent class="flex items-start justify-between pt-5">
          <div>
            <div class="text-sm text-muted-foreground">使用 IP 数</div>
            <div class="mt-1 font-display text-3xl font-semibold">{{ stats.unique_ips }}</div>
          </div>
          <div class="rounded-lg bg-primary/10 p-2 text-primary"><Network class="size-5" /></div>
        </CardContent>
      </Card>
      <Card>
        <CardContent class="flex items-start justify-between pt-5">
          <div>
            <div class="text-sm text-muted-foreground">进行中</div>
            <div class="mt-1 font-display text-3xl font-semibold">{{ stats.active_pulls }}</div>
          </div>
          <div class="rounded-lg bg-primary/10 p-2 text-primary"><Activity class="size-5" /></div>
        </CardContent>
      </Card>
    </div>

    <Card v-if="stats">
      <CardHeader>
        <CardTitle>最近拉取（本账号令牌）</CardTitle>
      </CardHeader>
      <CardContent class="overflow-x-auto">
        <table class="w-full text-sm">
          <thead class="text-left text-muted-foreground">
            <tr>
              <th class="pb-2">时间</th>
              <th class="pb-2">镜像</th>
              <th class="pb-2">IP</th>
              <th class="pb-2">流量</th>
              <th class="pb-2">状态</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="p in stats.recent_pulls" :key="p.id" class="border-t border-border">
              <td class="py-2 whitespace-nowrap">{{ formatTime(p.started_at) }}</td>
              <td class="py-2">{{ p.image_name }}:{{ p.tag }}</td>
              <td class="py-2 font-mono text-xs">{{ p.client_ip }}</td>
              <td class="py-2">{{ formatBytes(p.bytes_total) }}</td>
              <td class="py-2"><Badge variant="secondary">{{ p.status }}</Badge></td>
            </tr>
          </tbody>
        </table>
        <p v-if="!stats.recent_pulls?.length" class="py-6 text-center text-sm text-muted-foreground">暂无拉取记录</p>
      </CardContent>
    </Card>
  </div>
</template>
