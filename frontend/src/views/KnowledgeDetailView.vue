<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import api from '../api'
import MarkdownContent from '../components/MarkdownContent.vue'
import { timeAgo } from '../utils/time'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const id = computed(() => Number(route.params.id))
const article = ref(null)
const loading = ref(true)

onMounted(async () => {
  try {
    const { data } = await api.article(id.value)
    article.value = data
    api.viewArticle(id.value).catch(() => {})
  } catch (e) {
    article.value = null
  } finally {
    loading.value = false
  }
})

async function remove() {
  if (!confirm('确认删除该文章？')) return
  try {
    await api.deleteArticle(id.value)
    router.push('/knowledge')
  } catch (e) {
    alert(e.message)
  }
}
</script>

<template>
  <div v-if="loading" class="page"><p class="empty-note">加载中…</p></div>
  <div v-else-if="!article" class="page"><p class="empty-note">文章不存在或未发布</p></div>
  <div v-else class="page page-narrow">
    <div class="detail-card">
      <span class="badge badge-teal">知识文章</span>
      <h2>{{ article.title }}</h2>
      <p v-if="article.summary" style="color: var(--muted); margin-bottom: 14px">{{ article.summary }}</p>
      <MarkdownContent :source="article.body" />
      <div class="meta" style="margin-top: 18px">
        <span v-if="article.tags">
          <span v-for="t in article.tags.split(',').filter(Boolean)" :key="t" class="tag">{{ t.trim() }}</span>
        </span>
        <span style="margin-left: auto">
          {{ article.author }} · {{ timeAgo(article.created_at) }} · 浏览 {{ article.views }}
        </span>
      </div>
      <div class="actions-row" v-if="auth.isLoggedIn && (auth.user?.id === article.user_id || auth.isAdmin)">
        <router-link class="text-link" :to="`/knowledge/${article.id}/edit`">编辑</router-link>
        <button class="ghost-danger" @click="remove">删除文章</button>
      </div>
    </div>
  </div>
</template>
