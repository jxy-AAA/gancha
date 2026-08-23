<script setup>
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import api from '../api'
import Editor from '../components/Editor.vue'
import FileUpload from '../components/FileUpload.vue'

const router = useRouter()
const auth = useAuthStore()
const categories = ref([])
const categoryId = ref(0)
const title = ref('')
const body = ref('')
const tags = ref('')
const files = ref([])
const error = ref('')
const submitting = ref(false)

onMounted(async () => {
  if (!auth.isLoggedIn) return router.push('/login?redirect=/ask/new')
  try {
    const { data } = await api.publicCategories()
    categories.value = data.items
    categoryId.value = data.items[0]?.id || 0
  } catch (e) {
    error.value = e.message
  }
})

async function submit() {
  error.value = ''
  if (!categoryId.value) return (error.value = '请选择分类')
  if (!title.value.trim()) return (error.value = '标题不能为空')
  if (!body.value.trim()) return (error.value = '内容不能为空')
  submitting.value = true
  try {
    const { data } = await api.createQuestion({
      category_id: categoryId.value,
      title: title.value.trim(),
      body: body.value,
      tags: tags.value.trim(),
    })
    router.push(`/ask/${data.id}`)
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
      <p class="eyebrow">SUBMIT</p>
      <h2>发布投稿</h2>
      <label>投稿分类</label>
      <select v-model.number="categoryId">
        <option v-for="c in categories" :key="c.id" :value="c.id">{{ c.name }}</option>
      </select>
      <label>标题</label>
      <input v-model="title" type="text" maxlength="160" placeholder="请概括你的问题或分享内容" />
      <label>内容</label>
      <Editor v-model="body" placeholder="支持 Markdown 与 LaTeX 公式，例如：$f=1/\\Phi$" />
      <label>标签</label>
      <input v-model="tags" type="text" maxlength="250" placeholder="自行填写，用逗号分隔，例如：MTF, 像差, 双胶合" />
      <label>图片或资料附件</label>
      <FileUpload v-model="files" />
      <small class="upload-help">最多 5 个文件，每个不超过 10 MB。请勿上传公司机密或侵权资料。</small>
      <p class="form-hint">{{ error }}</p>
      <div class="form-footer">
        <button class="ghost-btn" @click="router.back()">取消</button>
        <button class="primary-btn" :disabled="submitting" @click="submit">发布投稿</button>
      </div>
    </div>
  </div>
</template>
