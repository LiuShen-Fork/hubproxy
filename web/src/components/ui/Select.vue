<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import type { HTMLAttributes } from 'vue'
import { Check, ChevronDown } from 'lucide-vue-next'
import { cn } from '@/lib/utils'

export type SelectOption = {
  value: string
  label: string
}

const model = defineModel<string>({ default: '' })

const props = withDefaults(
  defineProps<{
    options: SelectOption[]
    placeholder?: string
    disabled?: boolean
    class?: HTMLAttributes['class']
  }>(),
  {
    placeholder: '请选择',
  },
)

const open = ref(false)
const root = ref<HTMLElement | null>(null)
const panel = ref<HTMLElement | null>(null)
const trigger = ref<HTMLElement | null>(null)

const panelStyle = ref<Record<string, string>>({})

const selected = computed(() => props.options.find((o) => o.value === model.value))

function updatePosition() {
  const el = trigger.value
  if (!el) return
  const rect = el.getBoundingClientRect()
  const gap = 6
  const maxH = 280
  const spaceBelow = window.innerHeight - rect.bottom - gap
  const spaceAbove = rect.top - gap
  const openUp = spaceBelow < 160 && spaceAbove > spaceBelow

  panelStyle.value = {
    position: 'fixed',
    left: `${Math.max(8, rect.left)}px`,
    width: `${Math.max(rect.width, 120)}px`,
    maxHeight: `${Math.min(maxH, openUp ? spaceAbove : spaceBelow)}px`,
    zIndex: '9999',
    ...(openUp
      ? { bottom: `${window.innerHeight - rect.top + gap}px`, top: 'auto' }
      : { top: `${rect.bottom + gap}px`, bottom: 'auto' }),
  }
}

async function toggle() {
  if (props.disabled) return
  open.value = !open.value
  if (open.value) {
    await nextTick()
    updatePosition()
  }
}

function pick(value: string) {
  model.value = value
  open.value = false
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

watch(open, async (v) => {
  if (v) {
    await nextTick()
    updatePosition()
  }
})

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
      :disabled="disabled"
      :aria-expanded="open"
      class="flex h-11 w-full items-center justify-between gap-2 rounded-xl border border-input bg-background/80 px-3.5 text-left text-sm outline-none transition-[border-color,box-shadow,background-color] duration-150 hover:bg-accent/40 focus-visible:border-ring focus-visible:ring-2 focus-visible:ring-ring/40 disabled:cursor-not-allowed disabled:opacity-50"
      @click="toggle"
    >
      <span :class="selected ? 'text-foreground' : 'text-muted-foreground'">
        {{ selected?.label || placeholder }}
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
            v-for="opt in options"
            :key="opt.value"
            type="button"
            class="flex w-full items-center justify-between gap-2 rounded-lg px-3 py-2.5 text-left text-sm transition-colors"
            :class="
              opt.value === model
                ? 'bg-primary/10 text-primary'
                : 'text-foreground hover:bg-accent'
            "
            @click="pick(opt.value)"
          >
            <span>{{ opt.label }}</span>
            <Check v-if="opt.value === model" class="size-3.5 shrink-0" />
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
