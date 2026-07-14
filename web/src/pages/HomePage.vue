<script setup lang="ts">
import { computed, ref } from 'vue'
import { RouterLink } from 'vue-router'
import {
  BookOpen,
  Check,
  Clipboard,
  Container,
  Globe2,
  Link2,
  Rocket,
  Search,
  Shield,
  Sparkles,
  Zap,
} from 'lucide-vue-next'
import Button from '@/components/ui/Button.vue'
import Input from '@/components/ui/Input.vue'
import PageHero from '@/components/PageHero.vue'
import { copyText } from '@/lib/utils'
import { site } from '@/lib/site'

const input = ref('')
const output = ref('')
const error = ref('')
const copied = ref(false)

const host = computed(() => window.location.host)

const features = [
  { icon: Rocket, label: 'GitHub 加速' },
  { icon: Container, label: 'Docker 镜像' },
  { icon: Sparkles, label: 'Hugging Face' },
] as const

const highlights = [
  {
    icon: Zap,
    title: '多源统一加速',
    desc: 'Docker、GitHub、Hugging Face 一站接入，减少切换成本。',
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
    example: (h: string) => `docker pull ${h}/令牌/nginx:latest`,
    note: '官方 / 用户镜像',
  },
  {
    name: 'GHCR',
    domain: 'ghcr.io',
    example: (h: string) => `docker pull ${h}/令牌/ghcr.io/owner/app:tag`,
    note: 'GitHub Container Registry',
  },
  {
    name: 'GCR',
    domain: 'gcr.io',
    example: (h: string) => `docker pull ${h}/令牌/gcr.io/project/image:tag`,
    note: 'Google Container Registry',
  },
  {
    name: 'Quay',
    domain: 'quay.io',
    example: (h: string) => `docker pull ${h}/令牌/quay.io/org/image:tag`,
    note: 'Red Hat Quay',
  },
  {
    name: 'Kubernetes',
    domain: 'registry.k8s.io',
    example: (h: string) => `docker pull ${h}/令牌/registry.k8s.io/pause:3.9`,
    note: 'K8s 官方镜像',
  },
  {
    name: 'GitLab',
    domain: 'registry.gitlab.com',
    example: (h: string) => `docker pull ${h}/令牌/registry.gitlab.com/group/project:tag`,
    note: 'GitLab Container Registry',
  },
] as const

const githubSources = [
  { name: 'Release / Archive', sample: 'github.com/owner/repo/releases/...' },
  { name: 'Raw / Blob', sample: 'github.com/owner/repo/raw|blob/...' },
  { name: 'Git Clone', sample: 'github.com/owner/repo.git' },
  { name: 'GitHub API', sample: 'api.github.com/repos/owner/repo/...' },
  { name: 'Gist', sample: 'gist.github.com / gist.githubusercontent.com' },
  { name: 'Assets', sample: 'github.githubassets.com / opengraph.githubassets.com' },
  { name: 'Hugging Face', sample: 'huggingface.co / cdn-lfs.hf.co' },
] as const

const dockerExamples = computed(() => [
  {
    id: 'official',
    label: '官方镜像',
    original: 'docker pull nginx',
    accelerated: `docker pull ${host.value}/你的令牌/nginx`,
  },
  {
    id: 'user',
    label: '用户镜像',
    original: 'docker pull user/app:tag',
    accelerated: `docker pull ${host.value}/你的令牌/user/app:tag`,
  },
  {
    id: 'ghcr',
    label: 'GHCR',
    original: 'docker pull ghcr.io/org/app',
    accelerated: `docker pull ${host.value}/你的令牌/ghcr.io/org/app`,
  },
])

const allowedHosts = [
  'github.com/',
  'raw.githubusercontent.com/',
  'gist.githubusercontent.com/',
  'huggingface.co/',
  'cdn-lfs.hf.co/',
]

function formatLink() {
  error.value = ''
  copied.value = false
  const link = input.value.trim()
  if (!link) {
    error.value = '请输入有效的链接'
    output.value = ''
    return
  }

  if (link.startsWith('https://') || link.startsWith('http://')) {
    output.value = `https://${host.value}/${link}`
    return
  }

  if (allowedHosts.some((prefix) => link.startsWith(prefix))) {
    output.value = `https://${host.value}/https://${link}`
    return
  }

  error.value = '请输入有效的 GitHub / Hugging Face 链接'
  output.value = ''
}

async function onCopy() {
  if (!output.value) return
  copied.value = await copyText(output.value)
}

function onOpen() {
  if (!output.value) return
  window.open(output.value, '_blank', 'noopener,noreferrer')
}
</script>

<template>
  <div class="mx-auto max-w-4xl">
    <PageHero
      :eyebrow="site.fullName"
      :title="site.name"
      :subtitle="`Docker · GitHub · Hugging Face 多源加速 · ${site.tagline}`"
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
        <a href="#accelerate">
          <Button>立即加速</Button>
        </a>
        <RouterLink to="/search">
          <Button variant="outline">
            <Search class="size-4" />
            搜索镜像
          </Button>
        </RouterLink>
        <a :href="site.blog" target="_blank" rel="noopener noreferrer">
          <Button variant="outline">
            <BookOpen class="size-4" />
            站长博客
          </Button>
        </a>
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

    <section id="accelerate" class="surface-panel field-block">
      <div class="mb-4 space-y-1 text-center sm:text-left">
        <h2 class="font-display text-lg font-semibold">链接加速</h2>
        <p class="text-sm text-muted-foreground">粘贴 GitHub / Hugging Face 原始链接，一键生成加速地址</p>
      </div>
      <div class="flex flex-col gap-3 sm:flex-row">
        <Input
          v-model="input"
          class="sm:flex-1"
          placeholder="粘贴 GitHub / Hugging Face 原始链接"
          @keyup.enter="formatLink"
        />
        <Button class="shrink-0" @click="formatLink">获取加速链接</Button>
      </div>

      <Transition name="fade" mode="out-in">
        <p v-if="error" key="error" class="text-center text-destructive">{{ error }}</p>
        <div v-else-if="output" key="output" class="space-y-4 pt-2">
          <div class="flex items-center justify-center gap-2 font-medium text-primary">
            <Check class="size-4" />
            加速链接已生成
          </div>
          <p class="break-all rounded-lg border border-border bg-muted/40 px-4 py-3.5 font-mono">
            {{ output }}
          </p>
          <div class="flex flex-wrap justify-center gap-2">
            <Button variant="secondary" size="sm" @click="onCopy">
              <Clipboard class="size-4" />
              {{ copied ? '已复制' : '复制链接' }}
            </Button>
            <Button variant="secondary" size="sm" @click="onOpen">
              <Link2 class="size-4" />
              打开链接
            </Button>
          </div>
        </div>
      </Transition>
    </section>

    <section class="space-y-6 pt-12">
      <div class="space-y-1 text-center">
        <h2 class="text-sm font-semibold tracking-[0.16em] text-muted-foreground uppercase">
          支持的镜像源
        </h2>
        <p class="text-muted-foreground">
          当前程序内置并启用的 Registry 与文件加速源
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
                {{ item.example(host) }}
              </p>
            </div>
          </div>
          <p class="mt-3 text-xs text-muted-foreground">
            说明：登录控制台获取 8 位令牌。可用
            <code class="rounded bg-muted px-1">docker pull 域名/令牌/镜像</code>
            ，或在 daemon.json 配置
            <code class="rounded bg-muted px-1">registry-mirrors: ["https://域名/令牌"]</code>
            后直接 <code class="rounded bg-muted px-1">docker pull nginx</code>。仅支持匿名公开镜像。
          </p>
        </div>

        <div class="surface-panel rounded-xl border border-border/60 p-5 sm:col-span-2">
          <div class="mb-3 flex items-center gap-2">
            <Rocket class="size-4 text-primary" />
            <h3 class="font-display font-semibold">GitHub / Hugging Face</h3>
          </div>
          <div class="grid gap-2 sm:grid-cols-2">
            <div
              v-for="item in githubSources"
              :key="item.name"
              class="flex items-start justify-between gap-3 rounded-lg border border-border/70 bg-muted/20 px-3 py-2.5"
            >
              <span class="text-sm font-medium">{{ item.name }}</span>
              <span class="text-right font-mono text-[11px] text-muted-foreground">{{ item.sample }}</span>
            </div>
          </div>
          <p class="mt-3 text-xs text-muted-foreground">
            用法：在完整原始 URL 前加上本站域名，例如
            <span class="font-mono text-primary">https://{{ host }}/https://github.com/...</span>
          </p>
        </div>
      </div>
    </section>

    <section class="space-y-6 pt-12">
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
