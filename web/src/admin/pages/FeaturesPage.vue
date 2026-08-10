<script setup lang="ts">
import { onMounted, ref } from 'vue'
import Button from '@/components/ui/Button.vue'
import Card from '@/components/ui/Card.vue'
import CardContent from '@/components/ui/CardContent.vue'
import CardHeader from '@/components/ui/CardHeader.vue'
import CardTitle from '@/components/ui/CardTitle.vue'
import Switch from '@/components/ui/Switch.vue'
import { adminApi, type SettingsBundle } from '../api'
import { toastError, toastSuccess } from '@/lib/toast'

const settings = ref<SettingsBundle | null>(null)

async function load() {
  settings.value = await adminApi.settings()
}

async function saveFeatures() {
  if (!settings.value) return
  try {
    await adminApi.putFeatures(settings.value.features)
    toastSuccess('功能开关已保存')
    await load()
  } catch (e: any) {
    toastError(e?.message || '保存失败')
  }
}

async function saveRegistries() {
  if (!settings.value) return
  try {
    await adminApi.putRegistries(settings.value.registries)
    toastSuccess('Registry 开关已保存')
    await load()
  } catch (e: any) {
    toastError(e?.message || '保存失败')
  }
}

onMounted(async () => {
  try {
    await load()
  } catch (e: any) {
    toastError(e?.message || '加载失败')
  }
})
</script>

<template>
  <div class="space-y-4">

    <Card v-if="settings">
      <CardHeader>
        <CardTitle>加速路径开关</CardTitle>
        <p class="text-sm text-muted-foreground">精准关闭某一类能力，防止滥用</p>
      </CardHeader>
      <CardContent class="space-y-3">
        <div class="flex items-center justify-between rounded-lg border border-border p-4">
          <div>
            <div class="font-medium">Docker Hub</div>
            <div class="text-sm text-muted-foreground">官方 / 用户镜像拉取</div>
          </div>
          <Switch v-model:checked="settings.features.docker_hub" />
        </div>
        <div class="flex items-center justify-between rounded-lg border border-border p-4">
          <div>
            <div class="font-medium">GitHub 加速</div>
            <div class="text-sm text-muted-foreground">Release / Raw / Clone / API</div>
          </div>
          <Switch v-model:checked="settings.features.github" />
        </div>
        <div class="flex items-center justify-between rounded-lg border border-border p-4">
          <div>
            <div class="font-medium">Hugging Face</div>
            <div class="text-sm text-muted-foreground">模型与 LFS 文件</div>
          </div>
          <Switch v-model:checked="settings.features.huggingface" />
        </div>
        <div class="flex items-center justify-between rounded-lg border border-border p-4">
          <div>
            <div class="font-medium">镜像搜索</div>
            <div class="text-sm text-muted-foreground">Web / API 搜索 Docker Hub</div>
          </div>
          <Switch v-model:checked="settings.features.image_search" />
        </div>
        <div class="flex items-center justify-between rounded-lg border border-border p-4">
          <div>
            <div class="font-medium">离线镜像包</div>
            <div class="text-sm text-muted-foreground">打包 tar 下载</div>
          </div>
          <Switch v-model:checked="settings.features.offline_image" />
        </div>
        <div class="flex items-center justify-between rounded-lg border border-border p-4">
          <div>
            <div class="font-medium">开启公共镜像</div>
            <div class="text-sm text-muted-foreground">
              开启后可不带令牌直接 docker pull 域名/镜像；关闭后必须使用个人令牌路径
            </div>
          </div>
          <Switch v-model:checked="settings.features.public_mirror" />
        </div>
        <Button @click="saveFeatures">保存功能开关</Button>
      </CardContent>
    </Card>

    <Card v-if="settings">
      <CardHeader>
        <CardTitle>第三方 Registry 开关</CardTitle>
        <p class="text-sm text-muted-foreground">控制每个上游源是否允许通过本站代理</p>
      </CardHeader>
      <CardContent class="space-y-3">
        <div
          v-for="(reg, i) in settings.registries"
          :key="reg.domain"
          class="flex items-center justify-between gap-3 rounded-lg border border-border p-4"
        >
          <div class="min-w-0">
            <div class="font-medium">{{ reg.label || reg.domain }}</div>
            <div class="truncate font-mono text-xs text-muted-foreground">{{ reg.domain }}</div>
          </div>
          <Switch v-model:checked="settings.registries[i].enabled" />
        </div>
        <Button @click="saveRegistries">保存 Registry 开关</Button>
      </CardContent>
    </Card>
  </div>
</template>
