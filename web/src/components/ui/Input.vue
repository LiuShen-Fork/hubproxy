<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import { cn } from '@/lib/utils'

const model = defineModel<string | number>({ default: '' })

const props = defineProps<{
  class?: HTMLAttributes['class']
  type?: string
  id?: string
  placeholder?: string
  disabled?: boolean
  min?: number | string
  max?: number | string
  step?: number | string
  required?: boolean
  autocomplete?: string
  minlength?: number | string
  maxlength?: number | string
  readonly?: boolean
}>()
</script>

<template>
  <input
    :id="id"
    v-model="model"
    :type="type || 'text'"
    :placeholder="placeholder"
    :disabled="disabled"
    :readonly="readonly"
    :min="min"
    :max="max"
    :step="step"
    :required="required"
    :autocomplete="autocomplete"
    :minlength="minlength"
    :maxlength="maxlength"
    :class="cn(
      'h-11 w-full rounded-xl border border-input bg-background/70 px-3.5 text-sm outline-none transition-[border-color,box-shadow,background-color] duration-150 placeholder:text-muted-foreground/80 hover:bg-background focus-visible:border-ring focus-visible:bg-background focus-visible:ring-2 focus-visible:ring-ring/40 disabled:opacity-50',
      props.class,
    )"
  >
</template>
