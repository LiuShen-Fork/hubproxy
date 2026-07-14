<script setup lang="ts">
import { computed } from 'vue'
import { RouterView, useRoute } from 'vue-router'
import AppShell from '@/components/AppShell.vue'

const route = useRoute()
const isAdmin = computed(() => route.path.startsWith('/admin'))
</script>

<template>
  <RouterView v-if="isAdmin" />
  <AppShell v-else>
    <RouterView v-slot="{ Component, route: r }">
      <Transition name="page">
        <component :is="Component" :key="r.path" />
      </Transition>
    </RouterView>
  </AppShell>
</template>
