<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import { cn } from '@/lib/utils'

const props = withDefaults(
  defineProps<{
    variant?: 'default' | 'secondary' | 'outline' | 'success' | 'danger'
    class?: HTMLAttributes['class']
  }>(),
  { variant: 'default' },
)

const variants: Record<string, string> = {
  default: 'bg-primary text-primary-foreground',
  secondary: 'bg-secondary text-secondary-foreground',
  outline: 'border border-border text-foreground',
  success: 'bg-emerald-500/15 text-emerald-700 dark:text-emerald-300',
  danger: 'bg-destructive/15 text-destructive',
}
</script>

<template>
  <span
    :class="cn(
      'inline-flex items-center rounded-md px-2 py-0.5 text-xs font-medium',
      variants[variant],
      props.class,
    )"
  >
    <slot />
  </span>
</template>
