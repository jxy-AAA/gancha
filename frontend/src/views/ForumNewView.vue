<script setup>
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import api from '../api'
import Editor from '../components/Editor.vue'

const router = useRouter()
const auth = useAuthStore()
const boards = ref([])
const boardId = ref(0)
const title = ref('')
const body = ref('')
const tags = ref('')
const isAnonymous = ref(false)
const error = ref('')
const submitting = ref(false)

onMounted(async () => {
  if (!auth.isLoggedIn) return router.push('/login?redirect=/forum/new')
  try {
    const { data } = await api.boards()
    boards.value = data.items
    boardId.value = data.items[0]?.id || 0
  } catch (e) {
    error.value = e.message
  }
})

async function submit() {
  error.value = ''
  if (!boardId.value) return (error.value = '请选择板块')
  if (!title.value.trim()) return (error.value = '标题不能为空')
  if (!body.value.trim()) return (error.value = '内容不能为空')
  submitting.value = true
  try {
    const { data } = await api.createForumPost({
      board_id: boardId.value,
      title: title.value.trim(),
      body: body.value,
      tags: tags.value.trim(),
      is_anonymous: isAnonymous.value,
    })
    router.push(`/forum/${data.id}`)
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
      <p class="eyebrow">FORUM</p>
      <h2>发布新帖</h2>
      <label>板块</label>
      <select v-model.number="boardId">
        <option v-for="b in boards" :key="b.id" :value="b.id">{{ b.name }}</option>
      </select>
      <label>标题</label>
      <input v-model="title" type="text" maxlength="160" placeholder="帖子的标题" />
      <label>标签（可选，用逗号分隔）</label>
      <input v-model="tags" type="text" maxlength="250" placeholder="如：光学设计, 考研, 行业动态" />
      <label>内容</label>
      <Editor v-model="body" :rows="10" placeholder="自由讨论：行业动态、学术问题、资源交流…" />
      <label class="anon-option">
        <input v-model="isAnonymous" type="checkbox" />
        匿名发布（作者显示为「匿名」）
      </label>
      <p class="form-hint">{{ error }}</p>
      <div class="form-footer">
        <button class="ghost-btn" @click="router.back()">取消</button>
        <button class="primary-btn" :disabled="submitting" @click="submit">发布帖子</button>
      </div>
    </div>
  </div>
</template>
