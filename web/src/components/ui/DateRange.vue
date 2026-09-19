<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { HTMLAttributes } from 'vue'
import Input from './Input.vue'
import { cn } from '@/lib/utils'
import { dateInputToRFC3339, type DateRangePreset } from '@/lib/dateRange'

export type DateRangeValue = { preset: DateRangePreset; from: string; to: string }

const model = defineModel<DateRangeValue>({
  default: () => ({ preset: 'all', from: '', to: '' }),
})

const props = defineProps<{ class?: HTMLAttributes['class'] }>()

const presets: Array<{ value: DateRangePreset; label: string }> = [
  { value: 'today', label: '今天' },
  { value: '7d', label: '近 7 天' },
  { value: '30d', label: '近 30 天' },
  { value: 'all', label: '全部' },
  { value: 'custom', label: '自定义' },
]

// 自定义模式下用本地日期串驱动两个 date input
const fromDate = ref('')
const toDate = ref('')

const isCustom = computed(() => model.value.preset === 'custom')

watch(fromDate, () => {
  model.value = { ...model.value, from: dateInputToRFC3339(fromDate.value, false) }
})
watch(toDate, () => {
  model.value = { ...model.value, to: dateInputToRFC3339(toDate.value, true) }
})

function pick(preset: DateRangePreset) {
  if (preset === 'custom') {
    model.value = { preset, from: dateInputToRFC3339(fromDate.value, false), to: dateInputToRFC3339(toDate.value, true) }
    return
  }
  // from/to 由父组件调用 presetRange 计算，这里只切换预设，避免组件间职责重叠
  model.value = { preset, from: '', to: '' }
}
</script>

<template>
  <div :class="cn('space-y-2', props.class)">
    <div class="flex flex-wrap gap-1.5">
      <button
        v-for="p in presets"
        :key="p.value"
        type="button"
        class="rounded-lg border px-3 py-1.5 text-xs transition-colors"
        :class="
          model.preset === p.value
            ? 'border-primary bg-primary/10 text-primary'
            : 'border-input text-muted-foreground hover:bg-accent'
        "
        @click="pick(p.value)"
      >
        {{ p.label }}
      </button>
    </div>
    <div v-if="isCustom" class="flex items-center gap-2">
      <Input v-model="fromDate" type="date" class="h-10" />
      <span class="shrink-0 text-xs text-muted-foreground">至</span>
      <Input v-model="toDate" type="date" class="h-10" />
    </div>
  </div>
</template>
