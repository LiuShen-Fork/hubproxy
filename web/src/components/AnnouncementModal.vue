<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { X } from 'lucide-vue-next'
import Button from '@/components/ui/Button.vue'
import { site } from '@/lib/site'

const open = ref(false)
const html = ref('')

function storageKey(content: string) {
  // simple hash
  let h = 0
  for (let i = 0; i < content.length; i++) h = (h * 31 + content.charCodeAt(i)) | 0
  return `hubproxy_announcement_${h}`
}

function tryShow(content: string) {
  const c = (content || '').trim()
  if (!c) {
    open.value = false
    return
  }
  html.value = c
  try {
    if (localStorage.getItem(storageKey(c)) === '1') {
      open.value = false
      return
    }
  } catch {
    /* ignore */
  }
  open.value = true
}

function dismiss() {
  try {
    localStorage.setItem(storageKey(html.value), '1')
  } catch {
    /* ignore */
  }
  open.value = false
}

onMounted(() => tryShow(site.announcement || ''))
watch(
  () => site.announcement,
  (v) => tryShow(v || ''),
)
</script>

<template>
  <Teleport to="body">
    <div
      v-if="open"
      class="fixed inset-0 z-[9998] flex items-center justify-center bg-black/45 p-4 backdrop-blur-[2px]"
      @click.self="dismiss"
    >
      <div class="relative max-h-[80vh] w-full max-w-lg overflow-hidden rounded-2xl border border-border bg-background shadow-2xl">
        <div class="flex items-center justify-between border-b border-border px-5 py-3">
          <div class="font-display text-base font-semibold">站点公告</div>
          <button type="button" class="rounded-lg p-1 text-muted-foreground hover:bg-accent" @click="dismiss">
            <X class="size-4" />
          </button>
        </div>
        <div class="announcement-body max-h-[55vh] overflow-y-auto px-5 py-4 text-sm leading-relaxed" v-html="html" />
        <div class="border-t border-border px-5 py-3 text-right">
          <Button class="rounded-xl" size="sm" @click="dismiss">我知道了</Button>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
.announcement-body :deep(a) {
  color: var(--primary);
  text-decoration: underline;
}
.announcement-body :deep(p) {
  margin: 0.5em 0;
}
.announcement-body :deep(ul) {
  margin: 0.5em 0;
  padding-left: 1.25rem;
  list-style: disc;
}
</style>
