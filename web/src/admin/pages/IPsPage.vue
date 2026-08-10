<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import Button from '@/components/ui/Button.vue'
import Input from '@/components/ui/Input.vue'
import Card from '@/components/ui/Card.vue'
import CardContent from '@/components/ui/CardContent.vue'
import { adminApi, formatBytes, formatTime } from '../api'
import { useAuth } from '../auth'
import { useRouter } from 'vue-router'

const items = ref<any[]>([])
const total = ref(0)
const page = ref(1)
const ip = ref('')
const { isAdmin } = useAuth()
const router = useRouter()
const msg = ref('')

async function load() {
  const res = await adminApi.ips({ page: page.value, page_size: 20, ip: ip.value })
  items.value = res.items
  total.value = res.total
}

async function ban(ipAddr: string) {
  if (!isAdmin.value) return
  await adminApi.addBlackIP(ipAddr)
  msg.value = `已拉黑 ${ipAddr}`
}

onMounted(load)
watch(page, load)
const pages = () => Math.max(1, Math.ceil(total.value / 20))
</script>

<template>
  <div class="space-y-4">
    <p v-if="msg" class="text-sm text-emerald-600">{{ msg }}</p>
    <Card>
      <CardContent class="flex flex-col gap-3 pt-5 sm:flex-row">
        <Input v-model="ip" placeholder="按 IP 筛选" class="sm:max-w-xs" />
        <Button @click="page = 1; load()">查询</Button>
      </CardContent>
    </Card>
    <Card>
      <CardContent class="overflow-x-auto pt-5">
        <table class="w-full text-sm">
          <thead class="text-left text-muted-foreground">
            <tr>
              <th class="pb-2">IP</th>
              <th class="pb-2">拉取次数</th>
              <th class="pb-2">总流量</th>
              <th class="pb-2">最近活动</th>
              <th class="pb-2"></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="it in items" :key="it.client_ip" class="border-t border-border">
              <td class="py-2 font-mono text-xs">{{ it.client_ip }}</td>
              <td class="py-2">{{ it.pull_count }}</td>
              <td class="py-2">{{ formatBytes(it.bytes_total) }}</td>
              <td class="py-2">{{ formatTime(it.last_seen) }}</td>
              <td class="py-2 space-x-2">
                <Button size="sm" variant="ghost" @click="router.push({ path: '/admin/pulls', query: { ip: it.client_ip } })">
                  查看记录
                </Button>
                <Button v-if="isAdmin" size="sm" variant="outline" @click="ban(it.client_ip)">拉黑</Button>
              </td>
            </tr>
          </tbody>
        </table>
        <div class="mt-4 flex items-center justify-between text-sm">
          <span class="text-muted-foreground">共 {{ total }} 条</span>
          <div class="flex gap-2">
            <Button size="sm" variant="outline" :disabled="page <= 1" @click="page--">上一页</Button>
            <span class="px-2 py-1">{{ page }} / {{ pages() }}</span>
            <Button size="sm" variant="outline" :disabled="page >= pages()" @click="page++">下一页</Button>
          </div>
        </div>
      </CardContent>
    </Card>
  </div>
</template>
