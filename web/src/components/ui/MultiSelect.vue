<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref } from 'vue'
import type { HTMLAttributes } from 'vue'
import { Check, ChevronDown } from 'lucide-vue-next'
import { cn } from '@/lib/utils'
import type { SelectOption } from './Select.vue'

const model = defineModel<string[]>({ default: () => [] })

const props = withDefaults(
  defineProps<{
    options: SelectOption[]
    placeholder?: string
    class?: HTMLAttributes['class']
  }>(),
  { placeholder: '全部' },
)

const open = ref(false)
const root = ref<HTMLElement | null>(null)
const panel = ref<HTMLElement | null>(null)
const trigger = ref<HTMLElement | null>(null)
const panelStyle = ref<Record<string, string>>({})

const selectedLabels = computed(() =>
  props.options.filter((o) => model.value.includes(o.value)).map((o) => o.label),
)

function updatePosition() {
  const el = trigger.value
  if (!el) return
  const rect = el.getBoundingClientRect()
  const gap = 6
  const spaceBelow = window.innerHeight - rect.bottom - gap
  const spaceAbove = rect.top - gap
  const openUp = spaceBelow < 160 && spaceAbove > spaceBelow
  panelStyle.value = {
    position: 'fixed',
    left: `${Math.max(8, rect.left)}px`,
    width: `${Math.max(rect.width, 160)}px`,
    maxHeight: `${Math.min(280, openUp ? spaceAbove : spaceBelow)}px`,
    zIndex: '9999',
    ...(openUp
      ? { bottom: `${window.innerHeight - rect.top + gap}px`, top: 'auto' }
      : { top: `${rect.bottom + gap}px`, bottom: 'auto' }),
  }
}

async function toggle() {
  open.value = !open.value
  if (open.value) {
    await nextTick()
    updatePosition()
  }
}

function toggleValue(value: string) {
  const next = model.value.includes(value)
    ? model.value.filter((v) => v !== value)
    : [...model.value, value]
  model.value = next
}

function clear() {
  model.value = []
}

function onDocClick(e: MouseEvent) {
  const t = e.target as Node
  if (root.value?.contains(t) || panel.value?.contains(t)) return
  open.value = false
}

function onKey(e: KeyboardEvent) {
  if (e.key === 'Escape') open.value = false
}

function onReposition() {
  if (open.value) updatePosition()
}

onMounted(() => {
  document.addEventListener('click', onDocClick, true)
  document.addEventListener('keydown', onKey)
  window.addEventListener('resize', onReposition)
  window.addEventListener('scroll', onReposition, true)
})
onUnmounted(() => {
  document.removeEventListener('click', onDocClick, true)
  document.removeEventListener('keydown', onKey)
  window.removeEventListener('resize', onReposition)
  window.removeEventListener('scroll', onReposition, true)
})
</script>

<template>
  <div ref="root" :class="cn('relative', props.class)">
    <button
      ref="trigger"
      type="button"
      :aria-expanded="open"
      class="flex h-11 w-full items-center justify-between gap-2 rounded-xl border border-input bg-background/80 px-3.5 text-left text-sm outline-none transition-[border-color,box-shadow,background-color] duration-150 hover:bg-accent/40 focus-visible:border-ring focus-visible:ring-2 focus-visible:ring-ring/40"
      @click="toggle"
    >
      <span class="truncate" :class="selectedLabels.length ? 'text-foreground' : 'text-muted-foreground'">
        {{ selectedLabels.length ? selectedLabels.join('、') : placeholder }}
      </span>
      <ChevronDown
        class="size-4 shrink-0 text-muted-foreground transition-transform duration-200"
        :class="open ? 'rotate-180' : ''"
      />
    </button>

    <Teleport to="body">
      <Transition name="select-pop">
        <div
          v-if="open"
          ref="panel"
          :style="panelStyle"
          class="overflow-y-auto rounded-xl border border-border/80 bg-background p-1 shadow-xl shadow-black/10 dark:shadow-black/40"
        >
          <button
            type="button"
            class="mb-1 flex w-full items-center rounded-lg px-3 py-2 text-left text-sm text-muted-foreground transition-colors hover:bg-accent"
            @click="clear"
          >
            清空选择
          </button>
          <button
            v-for="opt in options"
            :key="opt.value"
            type="button"
            class="flex w-full items-center gap-2 rounded-lg px-3 py-2.5 text-left text-sm transition-colors"
            :class="model.includes(opt.value) ? 'bg-primary/10 text-primary' : 'text-foreground hover:bg-accent'"
            @click="toggleValue(opt.value)"
          >
            <span
              class="flex size-4 shrink-0 items-center justify-center rounded border"
              :class="model.includes(opt.value) ? 'border-primary bg-primary text-primary-foreground' : 'border-input'"
            >
              <Check v-if="model.includes(opt.value)" class="size-3" />
            </span>
            <span class="truncate">{{ opt.label }}</span>
          </button>
        </div>
      </Transition>
    </Teleport>
  </div>
</template>

<style scoped>
.select-pop-enter-active,
.select-pop-leave-active {
  transition: opacity 0.12s ease, transform 0.12s ease;
}
.select-pop-enter-from,
.select-pop-leave-to {
  opacity: 0;
  transform: translateY(-4px) scale(0.98);
}
</style>
