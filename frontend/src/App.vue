<script setup>
import { onMounted } from 'vue'
import { useAuthStore } from './stores/auth'
import Topbar from './components/Topbar.vue'
import SiteFooter from './components/SiteFooter.vue'

const auth = useAuthStore()
onMounted(() => {
  if (!auth.isLoggedIn) auth.fetchMe()
})
</script>

<template>
  <Topbar />
  <main>
    <router-view />
  </main>
  <SiteFooter />
</template>
