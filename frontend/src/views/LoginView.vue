<script setup>
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const router = useRouter()
const route = useRoute()
const auth = useAuthStore()
const email = ref('')
const password = ref('')
const error = ref('')
const loading = ref(false)

async function submit() {
  error.value = ''
  if (!email.value || !password.value) return (error.value = '请输入邮箱和密码')
  loading.value = true
  try {
    await auth.login(email.value, password.value)
    router.push(route.query.redirect || '/')
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="page page-narrow">
    <div class="form-card">
      <p class="eyebrow">WELCOME BACK</p>
      <h2>登录</h2>
      <label>邮箱</label>
      <input v-model="email" type="email" autocomplete="email" placeholder="name@example.com" @keyup.enter="submit" />
      <label>密码</label>
      <input v-model="password" type="password" autocomplete="current-password" placeholder="至少 8 位" @keyup.enter="submit" />
      <p class="form-hint">{{ error }}</p>
      <div class="form-footer">
        <router-link class="text-link" to="/register">没有账号？注册</router-link>
        <button class="primary-btn" :disabled="loading" @click="submit">登录</button>
      </div>
    </div>
  </div>
</template>
