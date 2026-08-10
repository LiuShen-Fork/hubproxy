<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { Copy } from 'lucide-vue-next'
import Button from '@/components/ui/Button.vue'
import Input from '@/components/ui/Input.vue'
import Label from '@/components/ui/Label.vue'
import Card from '@/components/ui/Card.vue'
import CardContent from '@/components/ui/CardContent.vue'
import CardHeader from '@/components/ui/CardHeader.vue'
import CardTitle from '@/components/ui/CardTitle.vue'
import Switch from '@/components/ui/Switch.vue'
import { adminApi, type SettingsBundle } from '../api'
import { applySiteFromApi } from '@/lib/site'
import { toastError, toastSuccess } from '@/lib/toast'
import { copyText } from '@/lib/utils'
import { useRouter } from 'vue-router'

const settings = ref<SettingsBundle | null>(null)
const redirectURL = ref('')
const testEmailTo = ref('')
const router = useRouter()

async function load() {
  const s = await adminApi.settings()
  if (s.admin && (s.admin as any).form_register_enabled == null) {
    s.admin.form_register_enabled = !!(s.admin as any).register_enabled
  }
  if (!s.site) {
    s.site = {
      name: '镜像加速',
      full_name: '',
      tagline: '',
      description: '',
      icp_text: '',
      police_text: '',
      announcement: '',
    } as any
  }
  if (!s.oauth) {
    s.oauth = {
      enabled: false,
      client_id: '',
      client_secret: '',
      auth_url: '',
      token_url: '',
      user_info_url: '',
      scopes: 'openid profile email',
      display_name: 'OAuth2 登录',
    } as any
  }
  if (!s.email) {
    s.email = {
      enabled: false,
      smtp_host: '',
      smtp_port: 587,
      username: '',
      password: '',
      from: '',
      from_name: 'HubProxy',
      use_tls: true,
    } as any
  }
  settings.value = s
  redirectURL.value = (s as any).oauth_redirect_url || `${location.origin}/api/admin/oauth/callback`
}

async function saveAdmin() {
  if (!settings.value) return
  try {
    await adminApi.putAdmin(settings.value.admin)
    toastSuccess('账号策略已保存')
  } catch (e: any) {
    toastError(e?.message || '保存失败')
  }
}

async function saveSite() {
  if (!settings.value) return
  try {
    await adminApi.putSite(settings.value.site as any)
    try {
      const pub = await adminApi.publicConfig()
      if (pub.site) applySiteFromApi(pub.site)
    } catch {
      /* ignore */
    }
    toastSuccess('站点信息已保存')
  } catch (e: any) {
    toastError(e?.message || '保存失败')
  }
}

async function saveOAuth() {
  if (!settings.value) return
  try {
    await adminApi.putOAuth(settings.value.oauth as any)
    toastSuccess('OAuth 配置已保存')
    await load()
  } catch (e: any) {
    toastError(e?.message || '保存失败')
  }
}

async function saveEmail() {
  if (!settings.value) return
  try {
    await adminApi.putEmail((settings.value as any).email)
    toastSuccess('邮件配置已保存')
    await load()
  } catch (e: any) {
    toastError(e?.message || '保存失败')
  }
}

async function testEmail() {
  try {
    const res = await adminApi.testEmail(testEmailTo.value)
    toastSuccess((res as any).message || '测试邮件已发送')
  } catch (e: any) {
    toastError(e?.message || '发送失败')
  }
}

async function copyRedirect() {
  if (await copyText(redirectURL.value)) toastSuccess('回调地址已复制')
}

const policePreview = computed(() => {
  const t = settings.value?.site?.police_text || ''
  if (!t) return ''
  const digits = t.replace(/\D/g, '')
  return digits
    ? `https://beian.mps.gov.cn/#/query/webSearch?code=${digits}`
    : 'https://beian.mps.gov.cn/'
})

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
        <CardTitle>注册与账号</CardTitle>
        <p class="text-sm text-muted-foreground">表单注册、OAuth2 登录/注册；绑定在启用 OAuth 后自动可用</p>
      </CardHeader>
      <CardContent class="space-y-3">
        <div class="flex items-center justify-between rounded-xl border border-border p-4">
          <div>
            <div class="font-medium">允许表单注册</div>
            <div class="text-sm text-muted-foreground">用户名 + 密码自助注册</div>
          </div>
          <Switch v-model:checked="settings.admin.form_register_enabled" />
        </div>
        <div class="flex items-center justify-between rounded-xl border border-border p-4">
          <div>
            <div class="font-medium">注册需邮箱验证</div>
            <div class="text-sm text-muted-foreground">需先配置下方 SMTP，并开启邮件服务</div>
          </div>
          <Switch v-model:checked="(settings.admin as any).email_register_enabled" />
        </div>
        <div class="flex items-center justify-between rounded-xl border border-border p-4">
          <div>
            <div class="font-medium">允许 OAuth2 登录</div>
            <div class="text-sm text-muted-foreground">已绑定用户可用 OAuth 登录</div>
          </div>
          <Switch v-model:checked="settings.admin.oauth_login_enabled" />
        </div>
        <div class="flex items-center justify-between rounded-xl border border-border p-4">
          <div>
            <div class="font-medium">允许 OAuth2 注册</div>
            <div class="text-sm text-muted-foreground">首次 OAuth 登录自动创建本地用户</div>
          </div>
          <Switch v-model:checked="settings.admin.oauth_register_enabled" />
        </div>
        <div class="flex flex-wrap gap-2">
          <Button class="rounded-xl" @click="saveAdmin">保存账号策略</Button>
          <Button variant="outline" class="rounded-xl" @click="router.push('/admin/change-password')">
            修改我的资料
          </Button>
        </div>
      </CardContent>
    </Card>

    <Card v-if="settings">
      <CardHeader>
        <CardTitle>站点信息</CardTitle>
        <p class="text-sm text-muted-foreground">名称、公告与备案号；页脚项目链接固定为 HubProxy</p>
      </CardHeader>
      <CardContent class="grid gap-3 sm:grid-cols-2">
        <div class="space-y-2">
          <Label>站点名称</Label>
          <Input v-model="settings.site.name" placeholder="如：我的镜像站" />
        </div>
        <div class="space-y-2">
          <Label>完整名称</Label>
          <Input v-model="settings.site.full_name" placeholder="副标题 / 全称" />
        </div>
        <div class="space-y-2 sm:col-span-2">
          <Label>标语</Label>
          <Input v-model="settings.site.tagline" />
        </div>
        <div class="space-y-2 sm:col-span-2">
          <Label>描述</Label>
          <Input v-model="settings.site.description" />
        </div>
        <div class="space-y-2">
          <Label>ICP 备案号</Label>
          <Input v-model="settings.site.icp_text" placeholder="空则不显示；链接自动为工信部备案站" />
        </div>
        <div class="space-y-2">
          <Label>公安备案号</Label>
          <Input v-model="settings.site.police_text" placeholder="空则不显示；链接自动生成查询地址" />
        </div>
        <p v-if="settings.site.police_text" class="sm:col-span-2 break-all text-xs text-muted-foreground">
          公安链接预览：{{ policePreview }}
        </p>
        <div class="space-y-2 sm:col-span-2">
          <Label>全局公告（HTML，空则不弹窗）</Label>
          <textarea
            v-model="(settings.site as any).announcement"
            rows="4"
            class="w-full rounded-xl border border-input bg-background/70 p-3 text-sm outline-none focus-visible:ring-2 focus-visible:ring-ring/40"
            placeholder="<p>欢迎使用本站</p>"
          />
        </div>
        <div class="sm:col-span-2">
          <Button class="rounded-xl" @click="saveSite">保存站点信息</Button>
        </div>
      </CardContent>
    </Card>

    <Card v-if="settings">
      <CardHeader>
        <CardTitle>OAuth2</CardTitle>
        <p class="text-sm text-muted-foreground">通用 OAuth2.0（Auth / Token / UserInfo）。启用后用户可在账户资料绑定。</p>
      </CardHeader>
      <CardContent class="space-y-3">
        <div class="flex items-center justify-between rounded-xl border border-border p-4">
          <div>
            <div class="font-medium">启用 OAuth2</div>
            <div class="text-sm text-muted-foreground">需填写 Client 与三个端点 URL</div>
          </div>
          <Switch v-model:checked="settings.oauth.enabled" />
        </div>
        <div class="space-y-2">
          <Label>回调地址（自动生成，请复制到身份提供商）</Label>
          <div class="flex gap-2">
            <Input :model-value="redirectURL" readonly class="font-mono text-xs" />
            <Button variant="outline" class="shrink-0 rounded-xl" @click="copyRedirect">
              <Copy class="size-4" />
              复制
            </Button>
          </div>
        </div>
        <div class="grid gap-3 sm:grid-cols-2">
          <div class="space-y-2">
            <Label>按钮名称</Label>
            <Input v-model="settings.oauth.display_name" placeholder="OAuth2 登录" />
          </div>
          <div class="space-y-2">
            <Label>Scopes</Label>
            <Input v-model="settings.oauth.scopes" placeholder="openid profile email" />
          </div>
          <div class="space-y-2">
            <Label>Client ID</Label>
            <Input v-model="settings.oauth.client_id" autocomplete="off" />
          </div>
          <div class="space-y-2">
            <Label>Client Secret</Label>
            <Input v-model="settings.oauth.client_secret" type="password" autocomplete="new-password" placeholder="********" />
          </div>
          <div class="space-y-2 sm:col-span-2">
            <Label>Authorization URL</Label>
            <Input v-model="settings.oauth.auth_url" placeholder="https://.../authorize" />
          </div>
          <div class="space-y-2 sm:col-span-2">
            <Label>Token URL</Label>
            <Input v-model="settings.oauth.token_url" placeholder="https://.../token" />
          </div>
          <div class="space-y-2 sm:col-span-2">
            <Label>UserInfo URL</Label>
            <Input v-model="settings.oauth.user_info_url" placeholder="https://.../userinfo" />
          </div>
        </div>
        <Button class="rounded-xl" @click="saveOAuth">保存 OAuth</Button>
      </CardContent>
    </Card>

    <Card v-if="settings">
      <CardHeader>
        <CardTitle>邮件 SMTP</CardTitle>
        <p class="text-sm text-muted-foreground">用于注册验证码与测试发信（无 OAuth 时可用邮箱管理注册）</p>
      </CardHeader>
      <CardContent class="space-y-3">
        <div class="flex items-center justify-between rounded-xl border border-border p-4">
          <div>
            <div class="font-medium">启用邮件服务</div>
          </div>
          <Switch v-model:checked="(settings as any).email.enabled" />
        </div>
        <div class="grid gap-3 sm:grid-cols-2">
          <div class="space-y-2">
            <Label>SMTP 主机</Label>
            <Input v-model="(settings as any).email.smtp_host" placeholder="smtp.example.com" />
          </div>
          <div class="space-y-2">
            <Label>端口</Label>
            <Input v-model.number="(settings as any).email.smtp_port" type="number" />
          </div>
          <div class="space-y-2">
            <Label>用户名</Label>
            <Input v-model="(settings as any).email.username" />
          </div>
          <div class="space-y-2">
            <Label>密码</Label>
            <Input v-model="(settings as any).email.password" type="password" placeholder="********" />
          </div>
          <div class="space-y-2">
            <Label>发件人 From</Label>
            <Input v-model="(settings as any).email.from" placeholder="noreply@example.com" />
          </div>
          <div class="space-y-2">
            <Label>发件人名称</Label>
            <Input v-model="(settings as any).email.from_name" />
          </div>
        </div>
        <div class="flex flex-col gap-2 sm:flex-row sm:items-center">
          <Input v-model="testEmailTo" class="sm:max-w-xs" placeholder="测试收件邮箱（可空=发件人）" />
          <Button variant="outline" class="rounded-xl" @click="testEmail">发送测试邮件</Button>
          <Button class="rounded-xl" @click="saveEmail">保存邮件配置</Button>
        </div>
      </CardContent>
    </Card>
  </div>
</template>
