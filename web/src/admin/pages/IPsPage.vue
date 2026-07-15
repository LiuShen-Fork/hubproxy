<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import Button from '@/components/ui/Button.vue'
import Input from '@/components/ui/Input.vue'
import Card from '@/components/ui/Card.vue'
import CardContent from '@/components/ui/CardContent.vue'
import DataTable from '@/components/ui/DataTable.vue'
import { adminApi, formatBytes, formatTime } from '../api'
import { useAuth } from '../auth'
import { useRouter } from 'vue-router'
import { toastError, toastSuccess } from '@/lib/toast'

const items = ref<any[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = 20
const ip = ref('')
const { isAdmin } = useAuth()
const router = useRouter()

async function load() {
  const res = await adminApi.ips({ page: page.value, page_size: pageSize, ip: ip.value })
  items.value = res.items
  total.value = res.total
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
      <CardContent class="flex flex-col gap-3 pt-5 sm:flex-row">
        <Input v-model="ip" placeholder="按 IP 筛选" class="sm:max-w-xs" />
        <Button class="rounded-xl" @click="page = 1; load()">查询</Button>
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
              <th class="px-3 py-2.5 font-medium whitespace-nowrap">拉取次数</th>
              <th class="px-3 py-2.5 font-medium whitespace-nowrap">总流量</th>
              <th class="px-3 py-2.5 font-medium whitespace-nowrap">最近活动</th>
              <th class="px-3 py-2.5 font-medium whitespace-nowrap">操作</th>
            </tr>
          </template>
          <tr v-for="it in items" :key="it.client_ip" class="border-t border-border/70">
            <td class="px-3 py-2.5 font-mono text-xs whitespace-nowrap">{{ it.client_ip }}</td>
            <td class="px-3 py-2.5 whitespace-nowrap">{{ it.pull_count }}</td>
            <td class="px-3 py-2.5 whitespace-nowrap">{{ formatBytes(it.bytes_total) }}</td>
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
        <p v-if="!items.length" class="py-6 text-center text-sm text-muted-foreground">暂无数据</p>
      </CardContent>
    </Card>
  </div>
</template>
