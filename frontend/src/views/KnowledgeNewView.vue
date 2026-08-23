<script setup>
import { onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import api from '../api'
import Editor from '../components/Editor.vue'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const editId = route.params.id ? Number(route.params.id) : null

const title = ref('')
const summary = ref('')
const body = ref('')
const tags = ref('')
const published = ref(true)
const error = ref('')
const submitting = ref(false)

onMounted(async () => {
  if (!auth.isLoggedIn) return router.push('/login?redirect=' + (editId ? `/knowledge/${editId}/edit` : '/knowledge/new'))
  if (editId) {
    try {
      const { data } = await api.article(editId)
      if (auth.user?.id !== data.user_id && !auth.isAdmin) return router.push(`/knowledge/${editId}`)
      title.value = data.title
      summary.value = data.summary
      body.value = data.body
      tags.value = data.tags
      published.value = data.published
    } catch (e) {
      error.value = e.message
    }
  }
})

async function submit() {
  error.value = ''
  if (!title.value.trim()) return (error.value = '标题不能为空')
  if (!body.value.trim()) return (error.value = '内容不能为空')
  submitting.value = true
  const payload = {
    title: title.value.trim(),
    summary: summary.value.trim(),
    body: body.value,
    tags: tags.value.trim(),
    published: published.value,
  }
  try {
    const { data } = editId
      ? await api.updateArticle(editId, payload)
      : await api.createArticle(payload)
    router.push(`/knowledge/${editId || data.id}`)
  } catch (e) {
    error.value = e.message
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <div class="page page-narrow">
    <div class="form-card">
      <p class="eyebrow">KNOWLEDGE</p>
      <h2>{{ editId ? '编辑文章' : '投稿知识文章' }}</h2>
      <label>标题</label>
      <input v-model="title" type="text" maxlength="160" placeholder="文章的标题" />
      <label>摘要</label>
      <input v-model="summary" type="text" maxlength="300" placeholder="一句话概括文章内容（选填）" />
      <label>内容</label>
      <Editor v-model="body" :rows="14" placeholder="支持 Markdown 与 LaTeX 公式" />
      <label>标签</label>
      <input v-model="tags" type="text" maxlength="250" placeholder="用逗号分隔，例如：成像光学, MTF, 公差分析" />
      <label style="display: flex; align-items: center; gap: 8px; margin-top: 16px">
        <input v-model="published" type="checkbox" style="width: auto" />
        立即发布（未勾选则仅自己和管理员可见）
      </label>
      <p class="form-hint">{{ error }}</p>
      <div class="form-footer">
        <button class="ghost-btn" @click="router.back()">取消</button>
        <button class="primary-btn" :disabled="submitting" @click="submit">
          {{ editId ? '保存修改' : '发布文章' }}
        </button>
      </div>
    </div>
  </div>
</template>
