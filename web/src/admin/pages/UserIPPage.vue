<script setup lang="ts">
import { onMounted, ref } from 'vue'
import Button from '@/components/ui/Button.vue'
import Input from '@/components/ui/Input.vue'
import Card from '@/components/ui/Card.vue'
import CardContent from '@/components/ui/CardContent.vue'
import CardHeader from '@/components/ui/CardHeader.vue'
import CardTitle from '@/components/ui/CardTitle.vue'
import Badge from '@/components/ui/Badge.vue'
import { adminApi } from '../api'

const items = ref<string[]>([])
const ip = ref('')
const msg = ref('')
const err = ref('')

async function load() {
  const res = await adminApi.userIPWhitelist()
  items.value = res.items || []
}

async function add() {
  if (!ip.value.trim()) return
  await adminApi.addUserIP(ip.value.trim())
  ip.value = ''
  msg.value = '已添加'
  await load()
}

async function remove(v: string) {
  await adminApi.removeUserIP(v)
  await load()
}

onMounted(async () => {
  try {
    await load()
  } catch (e: any) {
    err.value = e?.message || '加载失败'
  }
})
</script>

<template>
  <div class="space-y-4">
    <p v-if="msg" class="text-sm text-emerald-600">{{ msg }}</p>
    <p v-if="err" class="text-sm text-destructive">{{ err }}</p>
    <Card>
      <CardHeader>
        <CardTitle>我的 IP 白名单</CardTitle>
        <p class="text-sm text-muted-foreground">
          为空表示不限制 IP。配置后，只有列表中的 IP 可使用你的访问令牌拉取镜像。
        </p>
      </CardHeader>
      <CardContent class="space-y-3">
        <div class="flex flex-col gap-2 sm:flex-row">
          <Input v-model="ip" class="min-w-0 flex-1" placeholder="IP 或 CIDR，如 1.2.3.4 或 10.0.0.0/8" />
          <Button class="shrink-0" @click="add">添加</Button>
        </div>
        <div class="flex flex-wrap gap-2">
          <Badge v-for="v in items" :key="v" variant="success" class="gap-1">
            {{ v }}
            <button class="ml-1 opacity-70 hover:opacity-100" @click="remove(v)">×</button>
          </Badge>
          <span v-if="!items.length" class="text-sm text-muted-foreground">未配置（允许任意 IP 使用你的令牌）</span>
        </div>
      </CardContent>
    </Card>
  </div>
</template>
