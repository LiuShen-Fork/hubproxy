<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { Copy, RefreshCw } from 'lucide-vue-next'
import Button from '@/components/ui/Button.vue'
import Input from '@/components/ui/Input.vue'
import Card from '@/components/ui/Card.vue'
import CardContent from '@/components/ui/CardContent.vue'
import CardHeader from '@/components/ui/CardHeader.vue'
import CardTitle from '@/components/ui/CardTitle.vue'
import Badge from '@/components/ui/Badge.vue'
import { adminApi } from '../api'
import { copyText } from '@/lib/utils'

const token = ref('')
const examples = ref<Record<string, string>>({})
const notes = ref<string[]>([])
const requireToken = ref(true)
const msg = ref('')
const err = ref('')
const host = computed(() => window.location.host)

async function load() {
  const res = await adminApi.userToken()
  token.value = res.token?.token || ''
  examples.value = res.examples || {}
  requireToken.value = res.require_token
  const guide = await adminApi.userGuide()
  notes.value = guide.notes || []
  if (guide.examples) examples.value = guide.examples
}

async function reset() {
  if (!confirm('重置后旧令牌立即失效且不可复用，确认？')) return
  const res = await adminApi.resetUserToken()
  token.value = res.token?.token || ''
  examples.value = res.examples || {}
  msg.value = res.message || '已重置'
}

async function copy(text: string) {
  if (await copyText(text)) msg.value = '已复制'
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
        <CardTitle>我的访问令牌</CardTitle>
        <p class="text-sm text-muted-foreground">
          8 位字母数字，全局唯一。用于
          <code class="rounded bg-muted px-1">docker pull {{ host }}/令牌/镜像</code>
        </p>
      </CardHeader>
      <CardContent class="space-y-4">
        <div class="flex flex-col gap-2 sm:flex-row sm:items-center">
          <Input v-model="token" readonly class="font-mono text-lg tracking-widest" />
          <div class="flex shrink-0 gap-2">
            <Button variant="outline" @click="copy(token)"><Copy class="size-4" />复制</Button>
            <Button variant="outline" @click="reset"><RefreshCw class="size-4" />重置</Button>
          </div>
        </div>
        <Badge :variant="requireToken ? 'default' : 'secondary'">
          {{ requireToken ? '当前强制使用令牌路径' : '当前允许无令牌拉取（管理员可改）' }}
        </Badge>
      </CardContent>
    </Card>

    <Card>
      <CardHeader>
        <CardTitle>快捷配置 · 一键复制</CardTitle>
      </CardHeader>
      <CardContent class="space-y-3">
        <div
          v-for="(cmd, key) in examples"
          :key="key"
          class="flex flex-col gap-2 rounded-lg border border-border p-3 sm:flex-row sm:items-center"
        >
          <code class="min-w-0 flex-1 break-all font-mono text-xs sm:text-sm">{{ cmd }}</code>
          <Button size="sm" variant="outline" class="shrink-0" @click="copy(cmd)">复制</Button>
        </div>
      </CardContent>
    </Card>

    <Card>
      <CardHeader>
        <CardTitle>使用说明</CardTitle>
      </CardHeader>
      <CardContent>
        <ul class="list-disc space-y-2 pl-5 text-sm text-muted-foreground">
          <li v-for="(n, i) in notes" :key="i">{{ n }}</li>
        </ul>
        <div class="mt-4 space-y-3 rounded-lg border border-border bg-muted/30 p-4 text-sm">
          <div>
            <p class="font-medium text-foreground">方式 A · 显式拉取（任意环境）</p>
            <pre class="mt-2 overflow-x-auto rounded bg-background p-3 font-mono text-xs">docker pull {{ host }}/{{ token || '令牌' }}/nginx:latest</pre>
          </div>
          <div>
            <p class="font-medium text-foreground">方式 B · daemon.json 镜像加速（支持带令牌路径）</p>
            <p class="mt-1 text-muted-foreground">
              Docker 的 <code class="rounded bg-muted px-1">registry-mirrors</code>
              可以使用带路径的 URL。本站已支持将
              <code class="rounded bg-muted px-1">https://域名/令牌</code>
              作为 mirror，请求会落到
              <code class="rounded bg-muted px-1">/令牌/v2/...</code>。
            </p>
            <pre class="mt-2 overflow-x-auto rounded bg-background p-3 font-mono text-xs">{
  "registry-mirrors": ["https://{{ host }}/{{ token || '令牌' }}"]
}</pre>
            <p class="mt-2 text-xs text-muted-foreground">
              修改后执行 <code class="rounded bg-muted px-1">systemctl restart docker</code>，
              即可直接 <code class="rounded bg-muted px-1">docker pull nginx</code>。
              重置令牌后请同步更新 mirror 路径。
            </p>
          </div>
        </div>
      </CardContent>
    </Card>
  </div>
</template>
