<script setup lang="ts">
import { onMounted, ref } from 'vue'
import Button from '@/components/ui/Button.vue'
import Input from '@/components/ui/Input.vue'
import Label from '@/components/ui/Label.vue'
import Card from '@/components/ui/Card.vue'
import CardContent from '@/components/ui/CardContent.vue'
import CardHeader from '@/components/ui/CardHeader.vue'
import CardTitle from '@/components/ui/CardTitle.vue'
import Badge from '@/components/ui/Badge.vue'
import { adminApi, type SettingsBundle } from '../api'
import { toastError, toastSuccess } from '@/lib/toast'

const settings = ref<SettingsBundle | null>(null)
const blackIP = ref('')
const whiteIP = ref('')
const accessWhite = ref('')
const accessBlack = ref('')

async function load() {
  settings.value = await adminApi.settings()
  accessWhite.value = settings.value.access.white_list.join('\n')
  accessBlack.value = settings.value.access.black_list.join('\n')
}

async function saveRate() {
  if (!settings.value) return
  try {
    await adminApi.putRateLimit(settings.value.rate_limit)
    toastSuccess('限流配置已保存')
  } catch (e: any) {
    toastError(e?.message || '保存失败')
  }
}

async function savePullSession() {
  if (!settings.value) return
  try {
    await adminApi.putPullSession(settings.value.pull_session)
    toastSuccess('拉取会话配置已保存')
  } catch (e: any) {
    toastError(e?.message || '保存失败')
  }
}

async function addBlack() {
  if (!blackIP.value.trim()) return
  try {
    await adminApi.addBlackIP(blackIP.value.trim())
    blackIP.value = ''
    await load()
    toastSuccess('已加入黑名单')
  } catch (e: any) {
    toastError(e?.message || '操作失败')
  }
}

async function removeBlack(ip: string) {
  try {
    await adminApi.removeBlackIP(ip)
    await load()
    toastSuccess('已移除')
  } catch (e: any) {
    toastError(e?.message || '操作失败')
  }
}

async function addWhite() {
  if (!whiteIP.value.trim()) return
  try {
    await adminApi.addWhiteIP(whiteIP.value.trim())
    whiteIP.value = ''
    await load()
    toastSuccess('已加入白名单')
  } catch (e: any) {
    toastError(e?.message || '操作失败')
  }
}

async function removeWhite(ip: string) {
  try {
    await adminApi.removeWhiteIP(ip)
    await load()
    toastSuccess('已移除')
  } catch (e: any) {
    toastError(e?.message || '操作失败')
  }
}

async function saveAccess() {
  if (!settings.value) return
  try {
    const body = {
      ...settings.value.access,
      white_list: accessWhite.value.split(/\r?\n/).map((s) => s.trim()).filter(Boolean),
      black_list: accessBlack.value.split(/\r?\n/).map((s) => s.trim()).filter(Boolean),
    }
    await adminApi.putAccess(body)
    await load()
    toastSuccess('仓库访问控制已保存')
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

    <div v-if="settings" class="grid gap-4 lg:grid-cols-2">
      <Card>
        <CardHeader>
          <CardTitle>全局限流（HTTP）</CardTitle>
          <p class="text-sm text-muted-foreground">非 Docker 拉取接口的请求令牌桶限制</p>
        </CardHeader>
        <CardContent class="space-y-3">
          <div class="space-y-2">
            <Label>周期内请求上限</Label>
            <Input v-model.number="settings.rate_limit.request_limit" type="number" min="1" />
          </div>
          <div class="space-y-2">
            <Label>周期（小时）</Label>
            <Input v-model.number="settings.rate_limit.period_hours" type="number" min="0.1" step="0.1" />
          </div>
          <div class="space-y-2">
            <Label>完整镜像拉取上限 / IP / 周期</Label>
            <Input v-model.number="settings.rate_limit.pull_limit" type="number" min="0" />
            <p class="text-xs text-muted-foreground">按「一次完整拉取」计数，多层分片不重复计次。0 表示不限制。</p>
          </div>
          <Button @click="saveRate">保存限流</Button>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>拉取会话规则</CardTitle>
          <p class="text-sm text-muted-foreground">
            Manifest 只跟踪不计次 → 任意一层 Blob 成功计 1 次并结束本轮 → 再次 Manifest 开新一轮；
            纯 Manifest 探测超时后删除
          </p>
        </CardHeader>
        <CardContent class="space-y-3">
          <div class="space-y-2">
            <Label>会话匹配窗口（分钟）</Label>
            <Input v-model.number="settings.pull_session.window_minutes" type="number" min="1" />
            <p class="text-xs text-muted-foreground">未计数会话在窗口内把同镜像的 manifest/层拼在一起</p>
          </div>
          <div class="space-y-2">
            <Label>已计数会话空闲完成（分钟）</Label>
            <Input v-model.number="settings.pull_session.idle_minutes" type="number" min="1" />
          </div>
          <div class="space-y-2">
            <Label>Manifest 探测超时（秒）</Label>
            <Input
              v-model.number="settings.pull_session.manifest_probe_seconds"
              type="number"
              min="15"
              :placeholder="'60'"
            />
            <p class="text-xs text-muted-foreground">
              只有 manifest、一直没有 blob 时，超过该时间删除记录（默认 60 秒）
            </p>
          </div>
          <div class="space-y-2">
            <Label>拉取数据保留天数</Label>
            <Input
              v-model.number="settings.pull_session.retention_days"
              type="number"
              min="1"
              :placeholder="'90'"
            />
            <p class="text-xs text-muted-foreground">
              每天额度刷新后清理早于该天数的拉取会话、事件和日统计；用户账号不会删除
            </p>
          </div>
          <Button @click="savePullSession">保存会话配置</Button>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>IP 黑名单</CardTitle>
        </CardHeader>
        <CardContent class="space-y-3">
          <div class="flex flex-col gap-2 sm:flex-row sm:items-center">
            <Input
              v-model="blackIP"
              class="min-w-0 flex-1"
              placeholder="IP 或 CIDR，如 1.2.3.4 或 10.0.0.0/8"
            />
            <Button class="w-full shrink-0 sm:w-auto" @click="addBlack">添加</Button>
          </div>
          <div class="flex flex-wrap gap-2">
            <Badge v-for="ip in settings.security.black_list" :key="ip" variant="danger" class="gap-1">
              {{ ip }}
              <button class="ml-1 opacity-70 hover:opacity-100" @click="removeBlack(ip)">×</button>
            </Badge>
            <span v-if="!settings.security.black_list.length" class="text-sm text-muted-foreground">空</span>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>IP 白名单（限流豁免）</CardTitle>
        </CardHeader>
        <CardContent class="space-y-3">
          <div class="flex flex-col gap-2 sm:flex-row sm:items-center">
            <Input v-model="whiteIP" class="min-w-0 flex-1" placeholder="IP 或 CIDR" />
            <Button class="w-full shrink-0 sm:w-auto" @click="addWhite">添加</Button>
          </div>
          <div class="flex flex-wrap gap-2">
            <Badge v-for="ip in settings.security.white_list" :key="ip" variant="success" class="gap-1">
              {{ ip }}
              <button class="ml-1 opacity-70 hover:opacity-100" @click="removeWhite(ip)">×</button>
            </Badge>
            <span v-if="!settings.security.white_list.length" class="text-sm text-muted-foreground">空</span>
          </div>
        </CardContent>
      </Card>

      <Card class="lg:col-span-2">
        <CardHeader>
          <CardTitle>仓库 / 镜像访问控制</CardTitle>
          <p class="text-sm text-muted-foreground">每行一个，支持通配符，如 library/*、baduser/*</p>
        </CardHeader>
        <CardContent class="grid gap-4 md:grid-cols-2">
          <div class="space-y-2">
            <Label>白名单（空=不限制）</Label>
            <textarea v-model="accessWhite" rows="6" class="w-full rounded-lg border border-input bg-transparent p-3 font-mono text-sm" />
          </div>
          <div class="space-y-2">
            <Label>黑名单</Label>
            <textarea v-model="accessBlack" rows="6" class="w-full rounded-lg border border-input bg-transparent p-3 font-mono text-sm" />
          </div>
          <div class="space-y-2 md:col-span-2">
            <Label>出站 SOCKS5 代理</Label>
            <Input v-model="settings.access.proxy" placeholder="socks5://127.0.0.1:1080" />
          </div>
          <Button class="md:col-span-2 w-fit" @click="saveAccess">保存访问控制</Button>
        </CardContent>
      </Card>
    </div>
  </div>
</template>
