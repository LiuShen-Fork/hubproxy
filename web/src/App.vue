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
    <!-- 用 matched[0].path 而不是 r.path 作 key：
         /login 与 /register 指向同一个 AuthPage，必须换 key 强制重挂载，否则组件
         实例被复用、只更新 props，登录失败的 error 会残留在注册表单里；
         而 /admin/* 的子路由共用 AdminLayout 这个父记录，用 r.path 会让后台每次
         导航都整个重挂载（侧栏闪烁、移动端抽屉状态重置）。取 matched[0].path
         正好区分开：登录页各是自己的记录，后台各页都归到 /admin 同一条。 -->
    <component :is="Component" :key="r.matched[0].path" />
  </RouterView>
  <AppShell v-else>
    <RouterView v-slot="{ Component, route: r }">
      <Transition name="page">
        <component :is="Component" :key="r.path" />
      </Transition>
    </RouterView>
  </AppShell>
</template>
