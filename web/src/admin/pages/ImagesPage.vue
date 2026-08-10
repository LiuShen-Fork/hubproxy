<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import Button from '@/components/ui/Button.vue'
import Input from '@/components/ui/Input.vue'
import Select from '@/components/ui/Select.vue'
import Card from '@/components/ui/Card.vue'
import CardContent from '@/components/ui/CardContent.vue'
import Badge from '@/components/ui/Badge.vue'
import DataTable from '@/components/ui/DataTable.vue'
import { adminApi, formatBytes } from '../api'

const categoryOptions = [
  { value: '', label: '全部类别' },
  { value: 'library', label: 'library' },
  { value: 'user', label: 'user' },
]

const items = ref<any[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = 20
const image = ref('')
const category = ref('')
const registry = ref('')

async function load() {
  const res = await adminApi.images({
    page: page.value,
    page_size: pageSize,
    image: image.value,
    category: category.value,
    registry: registry.value,
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
      <CardContent class="grid gap-3 pt-5 md:grid-cols-4">
        <Input v-model="image" placeholder="镜像名称" />
        <Input v-model="registry" placeholder="Registry" />
        <Select v-model="category" :options="categoryOptions" />
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
              <th class="px-3 py-2.5 font-medium">镜像</th>
              <th class="px-3 py-2.5 font-medium">Registry</th>
              <th class="px-3 py-2.5 font-medium whitespace-nowrap">类别</th>
              <th class="px-3 py-2.5 font-medium whitespace-nowrap">拉取次数</th>
              <th class="px-3 py-2.5 font-medium whitespace-nowrap">独立 IP</th>
              <th class="px-3 py-2.5 font-medium whitespace-nowrap">总流量</th>
            </tr>
          </template>
          <tr v-for="it in items" :key="it.registry + it.image_name" class="border-t border-border/70">
            <td class="max-w-[12rem] truncate px-3 py-2.5 font-medium" :title="it.image_name">{{ it.image_name }}</td>
            <td class="max-w-[10rem] truncate px-3 py-2.5" :title="it.registry">{{ it.registry }}</td>
            <td class="px-3 py-2.5 whitespace-nowrap"><Badge variant="secondary">{{ it.category }}</Badge></td>
            <td class="px-3 py-2.5 whitespace-nowrap">{{ it.pull_count }}</td>
            <td class="px-3 py-2.5 whitespace-nowrap">{{ it.unique_ips }}</td>
            <td class="px-3 py-2.5 whitespace-nowrap">{{ formatBytes(it.bytes_total) }}</td>
          </tr>
        </DataTable>
        <p v-if="!items.length" class="py-6 text-center text-sm text-muted-foreground">暂无数据</p>
      </CardContent>
    </Card>
  </div>
</template>
