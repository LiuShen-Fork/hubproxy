<script setup lang="ts">
import { nextTick, onUnmounted, ref, watch } from 'vue'
import type { HTMLAttributes } from 'vue'
import { cn } from '@/lib/utils'

const model = defineModel<string>({ default: '' })

const props = withDefaults(
  defineProps<{
    /** 由调用方提供取数逻辑，组件本身不关心来源 */
    fetch: (q: string) => Promise<string[]>
    placeholder?: string
    class?: HTMLAttributes['class']
  }>(),
  { placeholder: '' },
)

const suggestions = ref<string[]>([])
const open = ref(false)
const active = ref(0)
let timer: ReturnType<typeof setTimeout> | undefined
let seq = 0

function schedule(q: string) {
  if (timer) clearTimeout(timer)
  // 前缀为空时不去请求——后端也会拒绝，这里省一次往返
  if (!q.trim()) {
    // 同时作废在途请求，否则清空输入后旧前缀的结果回来会把面板又弹开
    seq++
    suggestions.value = []
    open.value = false
    return
  }
  timer = setTimeout(async () => {
    const mine = ++seq
    try {
      const items = await props.fetch(q.trim())
      // 丢弃过期响应，避免慢请求覆盖新结果
      if (mine !== seq) return
      suggestions.value = items
      active.value = 0
      open.value = items.length > 0
    } catch {
      if (mine !== seq) return
      suggestions.value = []
      open.value = false
    }
  }, 200)
}

// 刚从建议里选中的值，不再当作新前缀重新请求（否则面板会自己再弹开）
let picked = ''

watch(model, (v) => {
  if (v === picked) {
    picked = ''
    return
  }
  schedule(v)
})

async function pick(v: string) {
  picked = v
  if (timer) clearTimeout(timer)
  seq++
  model.value = v
  open.value = false
  suggestions.value = []
  await nextTick()
}

function onKeydown(e: KeyboardEvent) {
  if (!open.value) return
  if (e.key === 'ArrowDown') {
    e.preventDefault()
    active.value = Math.min(active.value + 1, suggestions.value.length - 1)
  } else if (e.key === 'ArrowUp') {
    e.preventDefault()
    active.value = Math.max(active.value - 1, 0)
  } else if (e.key === 'Enter') {
    e.preventDefault()
    const v = suggestions.value[active.value]
    if (v) void pick(v)
  } else if (e.key === 'Escape') {
    open.value = false
  }
}

function onBlur() {
  // 延迟关闭，否则点击建议项时 blur 先触发、click 落空
  setTimeout(() => {
    open.value = false
  }, 150)
}

onUnmounted(() => {
  if (timer) clearTimeout(timer)
})
</script>

<template>
  <div :class="cn('relative', props.class)">
    <input
      v-model="model"
      type="text"
      :placeholder="placeholder"
      class="flex h-11 w-full rounded-xl border border-input bg-background/80 px-3.5 text-sm outline-none transition-[border-color,box-shadow] duration-150 focus-visible:border-ring focus-visible:ring-2 focus-visible:ring-ring/40"
      @keydown="onKeydown"
      @blur="onBlur"
      @focus="open = suggestions.length > 0"
    />
    <div
      v-if="open"
      class="absolute inset-x-0 top-full z-50 mt-1.5 max-h-64 overflow-y-auto rounded-xl border border-border/80 bg-background p-1 shadow-xl shadow-black/10 dark:shadow-black/40"
    >
      <button
        v-for="(s, i) in suggestions"
        :key="s"
        type="button"
        class="block w-full rounded-lg px-3 py-2 text-left font-mono text-xs transition-colors"
        :class="i === active ? 'bg-primary/10 text-primary' : 'text-foreground hover:bg-accent'"
        @mousedown.prevent="pick(s)"
      >
        {{ s }}
      </button>
    </div>
  </div>
</template>
