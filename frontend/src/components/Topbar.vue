<script setup>
import { computed } from 'vue'
import { useAuthStore } from '../stores/auth'
import icon from '../assets/guangyanji-icon.png'

const auth = useAuthStore()
const displayName = computed(() => auth.user?.username || '登录')
</script>

<template>
  <header class="topbar">
    <router-link class="brand" to="/">
      <img :src="icon" alt="棱语 OptiTalk" />
      <span>棱语 OptiTalk</span>
    </router-link>
    <nav>
      <router-link to="/ask">问答</router-link>
      <router-link to="/knowledge">知识库</router-link>
      <router-link to="/forum">论坛</router-link>
    </nav>
    <div class="top-actions">
      <router-link v-if="auth.isAdmin" class="ghost-btn" to="/admin">管理后台</router-link>
      <button v-if="!auth.isLoggedIn" class="ghost-btn" @click="$router.push('/login')">
        {{ displayName }}
      </button>
      <button v-else class="ghost-btn" @click="$router.push('/profile')">{{ displayName }}</button>
      <button class="primary-btn" @click="$router.push('/ask/new')">+ 提问题</button>
    </div>
  </header>
</template>
