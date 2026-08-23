<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const router = useRouter()
const auth = useAuthStore()
const username = ref('')
const email = ref('')
const password = ref('')
const error = ref('')
const loading = ref(false)

async function submit() {
  error.value = ''
  if (!username.value.trim()) return (error.value = '请输入用户名')
  if (!email.value) return (error.value = '请输入邮箱')
  if (password.value.length < 8) return (error.value = '密码至少 8 位')
  loading.value = true
  try {
    await auth.register(username.value.trim(), email.value, password.value)
    router.push('/')
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
      <p class="eyebrow">JOIN GUANGYANJI</p>
      <h2>注册</h2>
      <label>显示名称</label>
      <input v-model="username" type="text" autocomplete="nickname" maxlength="30" placeholder="可以使用中文名或昵称" />
      <label>邮箱</label>
      <input v-model="email" type="email" autocomplete="email" placeholder="name@example.com" />
      <label>密码</label>
      <input v-model="password" type="password" autocomplete="new-password" minlength="8" maxlength="72"
        placeholder="至少 8 位" />
      <p class="form-hint">{{ error }}</p>
      <div class="form-footer">
        <router-link class="text-link" to="/login">已有账号？登录</router-link>
        <button class="primary-btn" :disabled="loading" @click="submit">注册</button>
      </div>
    </div>
  </div>
</template>
