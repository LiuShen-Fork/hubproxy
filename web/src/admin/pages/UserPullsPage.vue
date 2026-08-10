<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import Button from '@/components/ui/Button.vue'
import Input from '@/components/ui/Input.vue'
import Card from '@/components/ui/Card.vue'
import CardContent from '@/components/ui/CardContent.vue'
import DataTable from '@/components/ui/DataTable.vue'
import { adminApi, formatBytes, formatTime, type PullSession } from '../api'

const items = ref<PullSession[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = 20
const ip = ref('')
const image = ref('')

async function load() {
  const res = await adminApi.userPulls({
    page: page.value,
    page_size: pageSize,
    ip: ip.value,
    image: image.value,
  })
  items.value = res.items
  total.value = res.total
}

onMounted(load)
watch(page, load)
</script>

<template>
  <div class="space-y-4">
    <Card>
      <CardContent class="grid gap-3 pt-5 md:grid-cols-3">
        <Input v-model="ip" placeholder="按 IP 筛选" />
        <Input v-model="image" placeholder="按镜像筛选" />
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
              <th class="px-3 py-2.5 font-medium whitespace-nowrap">时间</th>
              <th class="px-3 py-2.5 font-medium">镜像</th>
              <th class="px-3 py-2.5 font-medium">Registry</th>
              <th class="px-3 py-2.5 font-medium whitespace-nowrap">IP</th>
              <th class="px-3 py-2.5 font-medium whitespace-nowrap">流量</th>
            </tr>
          </template>
          <tr v-for="p in items" :key="p.id" class="border-t border-border/70">
            <td class="px-3 py-2.5 whitespace-nowrap">{{ formatTime(p.started_at) }}</td>
            <td class="max-w-[12rem] truncate px-3 py-2.5" :title="`${p.image_name}:${p.tag}`">
              {{ p.image_name }}:{{ p.tag }}
            </td>
            <td class="max-w-[8rem] truncate px-3 py-2.5" :title="p.registry">{{ p.registry }}</td>
            <td class="px-3 py-2.5 font-mono text-xs whitespace-nowrap">{{ p.client_ip }}</td>
            <td class="px-3 py-2.5 whitespace-nowrap">{{ formatBytes(p.bytes_total) }}</td>
          </tr>
        </DataTable>
        <p v-if="!items.length" class="py-6 text-center text-sm text-muted-foreground">暂无记录</p>
      </CardContent>
    </Card>
  </div>
</template>
