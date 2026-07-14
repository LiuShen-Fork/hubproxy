<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import { cn } from '@/lib/utils'

const props = defineProps<{ class?: HTMLAttributes['class'] }>()
</script>

<template>
  <div
    :class="cn(
      'rounded-2xl border border-border/80 bg-background/90 shadow-sm shadow-black/[0.03] backdrop-blur-sm dark:shadow-black/20',
      props.class,
    )"
  >
    <slot />
  </div>
</template>
