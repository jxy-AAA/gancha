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
const myVote = ref(false)
const comments = ref([])
const commentBody = ref('')
const error = ref('')

async function load() {
  loading.value = true
  try {
    const { data } = await api.article(id.value)
    article.value = data
    api.viewArticle(id.value).catch(() => {})
    const { data: cd } = await api.articleComments(id.value)
    comments.value = cd.items
  } catch (e) {
    article.value = null
    error.value = e.message
  } finally {
    loading.value = false
  }
}

async function loadMine() {
  if (!auth.isLoggedIn) return
  try {
    const { data } = await api.voteStatus({ target_type: 'article', target_id: id.value })
    myVote.value = data.voted
  } catch { /* 忽略 */ }
}

async function toggleVote() {
  if (!auth.isLoggedIn) return router.push('/login?redirect=' + route.fullPath)
  try {
    const { data } = await api.toggleVote({ target_type: 'article', target_id: id.value })
    myVote.value = data.voted
    article.value.score += data.voted ? 1 : -1
  } catch (e) {
    error.value = e.message
  }
}

async function submitComment() {
  if (!commentBody.value.trim()) return
  try {
    await api.createArticleComment(id.value, { body: commentBody.value })
    commentBody.value = ''
    const { data } = await api.articleComments(id.value)
    comments.value = data.items
    article.value.comment_count = comments.value.length
  } catch (e) {
    error.value = e.message
  }
}

async function deleteComment(c) {
  if (!confirm('确认删除该评论？')) return
  try {
    await api.deleteArticleComment(c.id)
    comments.value = comments.value.filter((x) => x.id !== c.id)
    article.value.comment_count = comments.value.length
  } catch (e) {
    error.value = e.message
  }
}

async function remove() {
  if (!confirm('确认删除该文章？')) return
  try {
    await api.deleteArticle(id.value)
    router.push('/knowledge')
  } catch (e) {
    alert(e.message)
  }
}

onMounted(() => {
  load()
  loadMine()
})
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
      <div class="actions-row">
        <button class="vote-btn" :class="{ active: myVote }" @click="toggleVote">
          ▲ 赞 {{ article.score }}
        </button>
        <span style="color: var(--muted); font-size: 12px">💬 {{ article.comment_count ?? comments.length }} 条评论</span>
        <router-link v-if="auth.isLoggedIn && (auth.user?.id === article.user_id || auth.isAdmin)" class="text-link"
          :to="`/knowledge/${article.id}/edit`" style="margin-left: auto">编辑</router-link>
        <button v-if="auth.isLoggedIn && (auth.user?.id === article.user_id || auth.isAdmin)" class="ghost-danger"
          @click="remove">删除文章</button>
      </div>
    </div>

    <div class="detail-card">
      <h2 style="font-size: 18px">评论 {{ comments.length }}</h2>
      <p v-if="error" class="form-hint" style="margin-bottom: 8px">{{ error }}</p>
      <div v-for="c in comments" :key="c.id" class="answer-item" style="padding: 10px 0">
        <div class="answer-head">
          <img v-if="c.avatar" :src="c.avatar" class="avatar" alt="" />
          <span>{{ c.author }}</span>
          <span style="margin-left: auto">{{ timeAgo(c.created_at) }}</span>
          <button v-if="auth.isLoggedIn && (auth.user?.id === c.user_id || auth.isAdmin)" class="ghost-danger"
            @click="deleteComment(c)">删除</button>
        </div>
        <p style="font-size: 14px">{{ c.body }}</p>
      </div>
      <div v-if="!comments.length" class="empty-note" style="padding: 20px 0">还没有评论，来写下第一条吧</div>
      <div v-if="auth.isLoggedIn" style="display: flex; gap: 10px; margin-top: 14px">
        <input v-model="commentBody" class="search-input" style="flex: 1; min-width: 0" maxlength="500"
          placeholder="写下评论…" @keyup.enter="submitComment" />
        <button class="primary-btn" @click="submitComment">评论</button>
      </div>
      <div v-else class="empty-note" style="padding: 14px 0">
        <router-link class="text-link" :to="`/login?redirect=${route.fullPath}`">登录后参与评论</router-link>
      </div>
    </div>
  </div>
</template>
