<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import Button from '@/components/ui/Button.vue'
import Input from '@/components/ui/Input.vue'
import Select from '@/components/ui/Select.vue'
import Card from '@/components/ui/Card.vue'
import CardContent from '@/components/ui/CardContent.vue'
import Badge from '@/components/ui/Badge.vue'
import { adminApi, formatBytes } from '../api'

const categoryOptions = [
  { value: '', label: '全部类别' },
  { value: 'library', label: 'library' },
  { value: 'user', label: 'user' },
]

const items = ref<any[]>([])
const total = ref(0)
const page = ref(1)
const image = ref('')
const category = ref('')
const registry = ref('')

async function load() {
  const res = await adminApi.images({
    page: page.value,
    page_size: 20,
    image: image.value,
    category: category.value,
    registry: registry.value,
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
      <CardContent class="grid gap-3 pt-5 md:grid-cols-4">
        <Input v-model="image" placeholder="镜像名称" />
        <Input v-model="registry" placeholder="Registry" />
        <Select v-model="category" :options="categoryOptions" />
        <Button class="rounded-xl" @click="page = 1; load()">查询</Button>
      </CardContent>
    </Card>
    <Card>
      <CardContent class="overflow-x-auto pt-5">
        <table class="w-full text-sm">
          <thead class="text-left text-muted-foreground">
            <tr>
              <th class="pb-2">镜像</th>
              <th class="pb-2">Registry</th>
              <th class="pb-2">类别</th>
              <th class="pb-2">拉取次数</th>
              <th class="pb-2">独立 IP</th>
              <th class="pb-2">总流量</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="it in items" :key="it.registry + it.image_name" class="border-t border-border">
              <td class="py-2 font-medium">{{ it.image_name }}</td>
              <td class="py-2">{{ it.registry }}</td>
              <td class="py-2"><Badge variant="secondary">{{ it.category }}</Badge></td>
              <td class="py-2">{{ it.pull_count }}</td>
              <td class="py-2">{{ it.unique_ips }}</td>
              <td class="py-2">{{ formatBytes(it.bytes_total) }}</td>
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
