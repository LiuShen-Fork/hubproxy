<script setup lang="ts">
import { computed, watch } from 'vue'
import type { HTMLAttributes } from 'vue'
import Button from '@/components/ui/Button.vue'
import { cn } from '@/lib/utils'

const props = withDefaults(
  defineProps<{
    minWidth?: string
    maxHeight?: string
    class?: HTMLAttributes['class']
    total?: number
    pageSize?: number
    paginate?: boolean
  }>(),
  {
    minWidth: '640px',
    maxHeight: '22rem',
    pageSize: 8,
    paginate: false,
    total: 0,
  },
)

const page = defineModel<number>('page', { default: 1 })

const totalPages = computed(() => {
  if (!props.paginate || props.pageSize <= 0) return 1
  return Math.max(1, Math.ceil((props.total || 0) / props.pageSize))
})

watch(
  () => [props.total, props.pageSize] as const,
  () => {
    if (page.value > totalPages.value) page.value = totalPages.value
    if (page.value < 1) page.value = 1
  },
)
</script>

<template>
  <div :class="cn('w-full min-w-0', props.class)">
    <div
      class="relative w-full overflow-auto overscroll-x-contain rounded-xl border border-border/60 [-webkit-overflow-scrolling:touch]"
      :style="{ maxHeight }"
    >
      <table class="w-full border-collapse text-left text-sm" :style="{ minWidth }">
        <thead class="sticky top-0 z-10 border-b border-border/80 bg-muted/95 text-muted-foreground shadow-sm backdrop-blur-sm">
          <slot name="head" />
        </thead>
        <tbody class="bg-background/50">
          <slot :page="page" :page-size="pageSize" :total-pages="totalPages" />
        </tbody>
      </table>
    </div>
    <div
      v-if="paginate && total > pageSize"
      class="mt-3 flex flex-wrap items-center justify-between gap-2 text-sm"
    >
      <span class="text-muted-foreground">共 {{ total }} 条</span>
      <div class="flex items-center gap-2">
        <Button
          size="sm"
          variant="outline"
          class="rounded-lg"
          :disabled="page <= 1"
          @click="page = Math.max(1, page - 1)"
        >
          上一页
        </Button>
        <span class="min-w-[4rem] text-center tabular-nums text-muted-foreground">
          {{ page }} / {{ totalPages }}
        </span>
        <Button
          size="sm"
          variant="outline"
          class="rounded-lg"
          :disabled="page >= totalPages"
          @click="page = Math.min(totalPages, page + 1)"
        >
          下一页
        </Button>
      </div>
    </div>
  </div>
</template>
