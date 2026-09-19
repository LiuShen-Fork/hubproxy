<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import Button from '@/components/ui/Button.vue'
import Autocomplete from '@/components/ui/Autocomplete.vue'
import DateRange from '@/components/ui/DateRange.vue'
import Badge from '@/components/ui/Badge.vue'
import Card from '@/components/ui/Card.vue'
import CardContent from '@/components/ui/CardContent.vue'
import DataTable from '@/components/ui/DataTable.vue'
import { presetRange, type DateRangePreset } from '@/lib/dateRange'
import { adminApi, formatBytes, formatTime, type IPStat } from '../api'
import { useAuth } from '../auth'
import { useRouter } from 'vue-router'
import { toastError, toastSuccess } from '@/lib/toast'

const items = ref<IPStat[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = 20
const ip = ref('')
const loading = ref(false)
const dateRange = ref<{ preset: DateRangePreset; from: string; to: string }>({
  preset: 'all',
  from: '',
  to: '',
})
const { isAdmin } = useAuth()
const router = useRouter()

async function load() {
  loading.value = true
  try {
    const range = presetRange(dateRange.value.preset)
    const res = await adminApi.ips({
      page: page.value,
      page_size: pageSize,
      ip: ip.value,
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

async function ban(ipAddr: string) {
  if (!isAdmin.value) return
  try {
    await adminApi.addBlackIP(ipAddr)
    toastSuccess(`已拉黑 ${ipAddr}`)
  } catch (e: any) {
    toastError(e?.message || '操作失败')
  }
}

onMounted(load)
watch(page, load)
</script>

<template>
  <div class="space-y-4">
    <Card>
      <CardContent class="flex flex-col gap-3 pt-5">
        <div class="flex flex-col gap-3 sm:flex-row">
          <Autocomplete
            v-model="ip"
            :fetch="(q) => adminApi.ipsSuggest(q).then((r) => r.items)"
            placeholder="按 IP 筛选（输入前几位会有建议）"
            class="sm:max-w-xs"
          />
          <Button class="rounded-xl" @click="search">查询</Button>
        </div>
        <DateRange v-model="dateRange" @update:model-value="search" />
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
              <th class="px-3 py-2.5 font-medium whitespace-nowrap">IP</th>
              <th class="px-3 py-2.5 font-medium whitespace-nowrap">用户</th>
              <th class="px-3 py-2.5 text-right font-medium whitespace-nowrap">拉取次数</th>
              <th class="px-3 py-2.5 text-right font-medium whitespace-nowrap">总流量</th>
              <th class="px-3 py-2.5 font-medium whitespace-nowrap">最近活动</th>
              <th class="px-3 py-2.5 font-medium whitespace-nowrap">操作</th>
            </tr>
          </template>
          <tr
            v-for="it in items"
            :key="it.client_ip"
            class="border-t border-border/70 transition-colors hover:bg-accent/40"
          >
            <td class="px-3 py-2.5 font-mono text-xs whitespace-nowrap">{{ it.client_ip }}</td>
            <td class="px-3 py-2.5 whitespace-nowrap">
              <template v-if="it.users.length">
                <span v-if="it.users.length > 1" class="mr-1.5 inline-block" title="多个用户共用同一 IP，值得留意">
                  <Badge variant="danger">{{ it.users.length }}</Badge>
                </span>
                <span :class="it.users.length > 1 ? 'text-destructive' : ''">{{ it.users.join('、') }}</span>
              </template>
              <span v-else class="text-muted-foreground">匿名</span>
            </td>
            <td class="px-3 py-2.5 text-right tabular-nums whitespace-nowrap">{{ it.pull_count }}</td>
            <td class="px-3 py-2.5 text-right tabular-nums whitespace-nowrap">{{ formatBytes(it.bytes_total) }}</td>
            <td class="px-3 py-2.5 whitespace-nowrap">{{ formatTime(it.last_seen) }}</td>
            <td class="px-3 py-2.5">
              <div class="flex flex-nowrap items-center gap-1">
                <Button
                  size="sm"
                  variant="ghost"
                  @click="router.push({ path: '/admin/pulls', query: { ip: it.client_ip } })"
                >
                  查看记录
                </Button>
                <Button v-if="isAdmin" size="sm" variant="outline" @click="ban(it.client_ip)">拉黑</Button>
              </div>
            </td>
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
