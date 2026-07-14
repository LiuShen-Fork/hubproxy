<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import Button from '@/components/ui/Button.vue'
import Input from '@/components/ui/Input.vue'
import Card from '@/components/ui/Card.vue'
import CardContent from '@/components/ui/CardContent.vue'
import Badge from '@/components/ui/Badge.vue'
import { adminApi, formatBytes, formatTime, type PullSession } from '../api'

const items = ref<PullSession[]>([])
const total = ref(0)
const page = ref(1)
const ip = ref('')
const image = ref('')

async function load() {
  const res = await adminApi.userPulls({
    page: page.value,
    page_size: 20,
    ip: ip.value,
    image: image.value,
  })
  items.value = res.items
  total.value = res.total
}

onMounted(load)
watch(page, load)
const pages = () => Math.max(1, Math.ceil(total.value / 20))
</script>

<template>
  <div class="space-y-4">
    <Card>
      <CardContent class="grid gap-3 pt-5 md:grid-cols-3">
        <Input v-model="ip" placeholder="按 IP 筛选" />
        <Input v-model="image" placeholder="按镜像筛选" />
        <Button @click="page = 1; load()">查询</Button>
      </CardContent>
    </Card>
    <Card>
      <CardContent class="overflow-x-auto pt-5">
        <table class="w-full text-sm">
          <thead class="text-left text-muted-foreground">
            <tr>
              <th class="pb-2">时间</th>
              <th class="pb-2">镜像</th>
              <th class="pb-2">Registry</th>
              <th class="pb-2">IP</th>
              <th class="pb-2">流量</th>
              <th class="pb-2">状态</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="p in items" :key="p.id" class="border-t border-border">
              <td class="py-2 whitespace-nowrap">{{ formatTime(p.started_at) }}</td>
              <td class="py-2">{{ p.image_name }}:{{ p.tag }}</td>
              <td class="py-2">{{ p.registry }}</td>
              <td class="py-2 font-mono text-xs">{{ p.client_ip }}</td>
              <td class="py-2">{{ formatBytes(p.bytes_total) }}</td>
              <td class="py-2"><Badge variant="outline">{{ p.status }}</Badge></td>
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
