<script setup lang="ts">
import { onMounted, ref } from 'vue'
import Button from '@/components/ui/Button.vue'
import Card from '@/components/ui/Card.vue'
import CardContent from '@/components/ui/CardContent.vue'
import CardHeader from '@/components/ui/CardHeader.vue'
import CardTitle from '@/components/ui/CardTitle.vue'
import Switch from '@/components/ui/Switch.vue'
import { adminApi, type SettingsBundle } from '../api'
import { useRouter } from 'vue-router'

const settings = ref<SettingsBundle | null>(null)
const msg = ref('')
const router = useRouter()

async function load() {
  settings.value = await adminApi.settings()
}

async function saveAdmin() {
  if (!settings.value) return
  await adminApi.putAdmin(settings.value.admin)
  msg.value = '已保存'
}

onMounted(load)
</script>

<template>
  <div class="space-y-4">
    <p v-if="msg" class="text-sm text-emerald-600">{{ msg }}</p>
    <Card v-if="settings">
      <CardHeader>
        <CardTitle>注册与账号</CardTitle>
      </CardHeader>
      <CardContent class="space-y-4">
        <div class="flex items-center justify-between rounded-lg border border-border p-4">
          <div>
            <div class="font-medium">开放用户注册</div>
            <div class="text-sm text-muted-foreground">默认关闭。开启后可在登录页自助注册（普通用户）。</div>
          </div>
          <Switch v-model:checked="settings.admin.register_enabled" />
        </div>
        <div class="flex gap-2">
          <Button @click="saveAdmin">保存</Button>
          <Button variant="outline" @click="router.push('/admin/change-password')">修改用户名 / 密码</Button>
        </div>
      </CardContent>
    </Card>
  </div>
</template>
