<script setup lang="ts">
import { CheckCircle2, Info, X, XCircle } from 'lucide-vue-next'
import { dismissToast, toasts, type ToastKind } from '@/lib/toast'

const icons: Record<ToastKind, typeof Info> = {
  success: CheckCircle2,
  error: XCircle,
  info: Info,
}

const styles: Record<ToastKind, string> = {
  success: 'border-emerald-500/30 bg-emerald-500/10 text-emerald-800 dark:text-emerald-200',
  error: 'border-destructive/30 bg-destructive/10 text-destructive',
  info: 'border-border bg-background/95 text-foreground',
}
</script>

<template>
  <div class="pointer-events-none fixed inset-x-0 top-4 z-[10000] flex flex-col items-center gap-2 px-4 sm:items-end sm:px-6">
    <TransitionGroup name="toast">
      <div
        v-for="t in toasts.items"
        :key="t.id"
        class="pointer-events-auto flex w-full max-w-sm items-start gap-3 rounded-xl border px-4 py-3 shadow-lg backdrop-blur-xl"
        :class="styles[t.kind]"
      >
        <component :is="icons[t.kind]" class="mt-0.5 size-4 shrink-0 opacity-90" />
        <div class="min-w-0 flex-1">
          <div v-if="t.title" class="text-sm font-semibold">{{ t.title }}</div>
          <div class="text-sm leading-relaxed opacity-90">{{ t.message }}</div>
        </div>
        <button type="button" class="rounded-md p-0.5 opacity-60 hover:opacity-100" @click="dismissToast(t.id)">
          <X class="size-3.5" />
        </button>
      </div>
    </TransitionGroup>
  </div>
</template>

<style scoped>
.toast-enter-active,
.toast-leave-active {
  transition: all 0.22s ease;
}
.toast-enter-from,
.toast-leave-to {
  opacity: 0;
  transform: translateY(-8px) scale(0.98);
}
</style>
