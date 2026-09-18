<script setup lang="ts">
import { computed } from 'vue'
import { RouterView, useRoute } from 'vue-router'
import AppShell from '@/components/AppShell.vue'
import ToastHost from '@/components/ToastHost.vue'
import AnnouncementModal from '@/components/AnnouncementModal.vue'

const route = useRoute()
// 全屏独立页面（控制台、登录、注册）不套站点外壳，直接整屏渲染
const isBare = computed(() => route.meta.bare === true)
</script>

<template>
  <ToastHost />
  <AnnouncementModal />
  <RouterView v-if="isBare" v-slot="{ Component, route: r }">
    <!-- /login 与 /register 共用 AuthPage，按 path 加 key 强制重挂载，
         否则组件实例被复用、只更新 props，登录失败的 error 会残留在注册表单里 -->
    <component :is="Component" :key="r.path" />
  </RouterView>
  <AppShell v-else>
    <RouterView v-slot="{ Component, route: r }">
      <Transition name="page">
        <component :is="Component" :key="r.path" />
      </Transition>
    </RouterView>
  </AppShell>
</template>
