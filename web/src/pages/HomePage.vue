<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import {
  Container,
  Globe2,
  Search,
  Shield,
  Zap,
} from 'lucide-vue-next'
import Button from '@/components/ui/Button.vue'
import PageHero from '@/components/PageHero.vue'
import { site } from '@/lib/site'
import { adminApi, getToken } from '@/admin/api'
import { setAccessToken } from '@/lib/accessToken'

const accessToken = ref('')
const authenticated = ref(false)
const runtimeFeatures = ref({
  docker_hub: true,
  image_search: true,
  public_mirror: false,
})

const host = computed(() => window.location.host)
const tokenLabel = computed(() => accessToken.value || '令牌')
const requireToken = computed(() => !runtimeFeatures.value.public_mirror)

const features = [
  { icon: Container, label: 'Docker 镜像' },
  { icon: Zap, label: '多源 Registry' },
] as const

const highlights = [
  {
    icon: Zap,
    title: '多源统一加速',
    desc: 'Docker Hub、GHCR、GCR、Quay 等 Registry 一站接入，减少切换成本。',
  },
  {
    icon: Shield,
    title: '自托管可控',
    desc: '数据与带宽自主掌握，后台可管理限流、黑白名单与统计。',
  },
  {
    icon: Globe2,
    title: '仅供自用',
    desc: '清羽飞扬个人自建服务，面向学习与日常开发场景。',
  },
] as const

const dockerRegistries = [
  {
    name: 'Docker Hub',
    domain: 'registry-1.docker.io',
    example: (h: string, t: string) => `docker pull ${h}/${t}/nginx:latest`,
    note: '官方 / 用户镜像',
  },
  {
    name: 'GHCR',
    domain: 'ghcr.io',
    example: (h: string, t: string) => `docker pull ${h}/${t}/ghcr.io/owner/app:tag`,
    note: 'GitHub Container Registry',
  },
  {
    name: 'GCR',
    domain: 'gcr.io',
    example: (h: string, t: string) => `docker pull ${h}/${t}/gcr.io/project/image:tag`,
    note: 'Google Container Registry',
  },
  {
    name: 'Quay',
    domain: 'quay.io',
    example: (h: string, t: string) => `docker pull ${h}/${t}/quay.io/org/image:tag`,
    note: 'Red Hat Quay',
  },
  {
    name: 'Kubernetes',
    domain: 'registry.k8s.io',
    example: (h: string, t: string) => `docker pull ${h}/${t}/registry.k8s.io/pause:3.9`,
    note: 'K8s 官方镜像',
  },
  {
    name: 'GitLab',
    domain: 'registry.gitlab.com',
    example: (h: string, t: string) => `docker pull ${h}/${t}/registry.gitlab.com/group/project:tag`,
    note: 'GitLab Container Registry',
  },
] as const

const dockerExamples = computed(() => [
  {
    id: 'official',
    label: '官方镜像',
    original: 'docker pull nginx',
    accelerated: `docker pull ${host.value}/${tokenLabel.value}/nginx`,
  },
  {
    id: 'user',
    label: '用户镜像',
    original: 'docker pull user/app:tag',
    accelerated: `docker pull ${host.value}/${tokenLabel.value}/user/app:tag`,
  },
  {
    id: 'ghcr',
    label: 'GHCR',
    original: 'docker pull ghcr.io/org/app',
    accelerated: `docker pull ${host.value}/${tokenLabel.value}/ghcr.io/org/app`,
  },
])

onMounted(async () => {
  try {
    const data = await adminApi.publicConfig()
    if (data.features) runtimeFeatures.value = { ...runtimeFeatures.value, ...data.features }
  } catch {
    // use defaults
  }

  if (!getToken()) return
  try {
    await adminApi.me()
    const res = await adminApi.userToken()
    accessToken.value = res.token?.token || ''
    authenticated.value = true
    setAccessToken(accessToken.value)
  } catch {
    // A stale admin token must not unlock private acceleration links.
    accessToken.value = ''
    authenticated.value = false
  }
})
</script>

<template>
  <div class="mx-auto max-w-4xl">
    <PageHero
      :eyebrow="site.fullName"
      :title="site.name"
      :subtitle="`Docker 多源镜像加速 · ${site.tagline}`"
      gradient
    >
      <div class="flex flex-wrap justify-center gap-2 pt-2">
        <span
          v-for="item in features"
          :key="item.label"
          class="feature-pill"
        >
          <component :is="item.icon" class="size-4" />
          {{ item.label }}
        </span>
      </div>
      <div class="flex flex-wrap justify-center gap-2 pt-4">
        <a href="#docker-pull">
          <Button>立即拉取</Button>
        </a>
        <RouterLink to="/search">
          <Button variant="outline">
            <Search class="size-4" />
            搜索镜像
          </Button>
        </RouterLink>
      </div>
    </PageHero>

    <section class="mb-10 grid gap-3 sm:grid-cols-3">
      <div
        v-for="item in highlights"
        :key="item.title"
        class="surface-panel rounded-xl border border-border/60 p-5"
      >
        <div class="mb-3 flex size-10 items-center justify-center rounded-lg bg-primary/10 text-primary">
          <component :is="item.icon" class="size-5" />
        </div>
        <h3 class="font-display text-base font-semibold">{{ item.title }}</h3>
        <p class="mt-1.5 text-sm leading-relaxed text-muted-foreground">{{ item.desc }}</p>
      </div>
    </section>

    <section class="space-y-6 pt-12">
      <div class="space-y-1 text-center">
        <h2 class="text-sm font-semibold tracking-[0.16em] text-muted-foreground uppercase">
          支持的镜像源
        </h2>
        <p class="text-muted-foreground">
          当前程序内置并启用的 Registry 加速源
        </p>
      </div>

      <div class="grid gap-3 sm:grid-cols-2">
        <div class="surface-panel rounded-xl border border-border/60 p-5 sm:col-span-2">
          <div class="mb-3 flex items-center gap-2">
            <Container class="size-4 text-primary" />
            <h3 class="font-display font-semibold">Docker Registry</h3>
          </div>
          <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
            <div
              v-for="item in dockerRegistries"
              :key="item.domain"
              class="rounded-lg border border-border/70 bg-muted/20 p-3"
            >
              <div class="font-medium">{{ item.name }}</div>
              <div class="mt-0.5 font-mono text-xs text-muted-foreground">{{ item.domain }}</div>
              <p class="mt-2 text-xs text-muted-foreground">{{ item.note }}</p>
              <p class="mt-2 break-all font-mono text-[11px] leading-relaxed text-primary/90">
                {{ item.example(host, tokenLabel) }}
              </p>
            </div>
          </div>
          <p class="mt-3 text-xs text-muted-foreground">
            说明：登录控制台获取 8 位令牌。可用
            <code class="rounded bg-muted px-1">docker pull 域名/令牌/镜像</code>
            ，或在 daemon.json 配置
            <code class="rounded bg-muted px-1">registry-mirrors: ["https://域名/令牌"]</code>
            后直接 <code class="rounded bg-muted px-1">docker pull nginx</code>。仅支持匿名公开镜像。
            <span v-if="requireToken">当前公共加速关闭，主页会优先使用本地保存的访问令牌生成链接。</span>
          </p>
        </div>
      </div>
    </section>

    <section id="docker-pull" class="space-y-6 pt-12">
      <div class="space-y-1 text-center">
        <h2 class="text-sm font-semibold tracking-[0.16em] text-muted-foreground uppercase">
          Docker 镜像加速
        </h2>
        <p class="text-muted-foreground">
          在镜像名前加上本站域名，一行命令即可加速拉取
        </p>
      </div>

      <div class="terminal-block">
        <div class="terminal-header">
          <span class="terminal-dot" />
          <span class="terminal-dot" />
          <span class="terminal-dot" />
          <span class="ml-2 text-xs text-muted-foreground">shell</span>
        </div>
        <div class="terminal-body">
          <div
            v-for="item in dockerExamples"
            :key="item.id"
            class="terminal-example"
          >
            <span class="example-tag">{{ item.label }}</span>
            <p class="font-mono leading-relaxed">
              <span class="text-muted-foreground">$ </span>
              <span class="text-muted-foreground/70 line-through decoration-muted-foreground/40">{{ item.original }}</span>
            </p>
            <p class="font-mono leading-relaxed">
              <span class="text-muted-foreground">$ </span>
              <span class="text-primary">{{ item.accelerated }}</span>
            </p>
          </div>
        </div>
      </div>
    </section>
  </div>
</template>
