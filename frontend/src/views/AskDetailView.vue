<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import api from '../api'
import MarkdownContent from '../components/MarkdownContent.vue'
import Editor from '../components/Editor.vue'
import FileUpload from '../components/FileUpload.vue'
import { timeAgo } from '../utils/time'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const qid = computed(() => Number(route.params.id))

const q = ref(null)
const answers = ref([])
const comments = ref([])
const myVote = ref(false)
const bookmarked = ref(false)
const following = ref(false)
const answerBody = ref('')
const commentBody = ref('')
const answerFiles = ref([])
const error = ref('')
const loading = ref(true)
const editingAnswer = ref(null)
const editBody = ref('')

async function load() {
  loading.value = true
  try {
    const [qd, ad, cd] = await Promise.all([
      api.question(qid.value),
      api.answers(qid.value),
      api.comments(qid.value),
    ])
    q.value = qd.data
    answers.value = ad.data.items
    comments.value = cd.data.items
    api.viewQuestion(qid.value).catch(() => {})
  } finally {
    loading.value = false
  }
}

async function loadMine() {
  if (!auth.isLoggedIn) return
  try {
    const { data } = await api.voteStatus({ target_type: 'question', target_id: qid.value })
    myVote.value = data.voted
    const { data: bd } = await api.bookmarks()
    bookmarked.value = bd.items.some((b) => b.id === qid.value)
  } catch {
    /* 忽略 */
  }
}

async function toggleVote(targetType, targetId) {
  if (!auth.isLoggedIn) return router.push('/login')
  try {
    const { data } = await api.toggleVote({ target_type: targetType, target_id: targetId })
    if (targetType === 'question') {
      myVote.value = data.voted
      q.value.score += data.voted ? 1 : -1
    } else {
      const a = answers.value.find((x) => x.id === targetId)
      if (a) {
        a.voted = data.voted
        a.score += data.voted ? 1 : -1
      }
    }
  } catch (e) {
    error.value = e.message
  }
}

async function toggleBookmark() {
  if (!auth.isLoggedIn) return router.push('/login')
  try {
    const { data } = await api.toggleBookmark(qid.value)
    bookmarked.value = data.bookmarked
  } catch (e) {
    error.value = e.message
  }
}
async function toggleFollow() {
  if (!auth.isLoggedIn) return router.push('/login')
  try {
    const { data } = await api.toggleFollow(qid.value)
    following.value = data.following
  } catch (e) {
    error.value = e.message
  }
}

async function submitAnswer() {
  error.value = ''
  if (!answerBody.value.trim()) return (error.value = '回答内容不能为空')
  try {
    await api.createAnswer(qid.value, { body: answerBody.value })
    answerBody.value = ''
    answerFiles.value = []
    load()
  } catch (e) {
    error.value = e.message
  }
}

async function acceptAnswer(id) {
  try {
    await api.acceptAnswer(id)
    load()
  } catch (e) {
    error.value = e.message
  }
}

function startEditAnswer(a) {
  editingAnswer.value = a
  editBody.value = a.body
}
async function saveEditAnswer() {
  try {
    await api.updateAnswer(editingAnswer.value.id, { body: editBody.value })
    editingAnswer.value = null
    load()
  } catch (e) {
    error.value = e.message
  }
}
async function deleteAnswer(a) {
  if (!confirm('确认删除该回答？')) return
  try {
    await api.deleteAnswer(a.id)
    load()
  } catch (e) {
    error.value = e.message
  }
}

async function submitComment() {
  if (!commentBody.value.trim()) return
  try {
    await api.createComment(qid.value, { body: commentBody.value })
    commentBody.value = ''
    const { data } = await api.comments(qid.value)
    comments.value = data.items
  } catch (e) {
    error.value = e.message
  }
}

async function deleteComment(id) {
  if (!confirm('确认删除该评论？')) return
  try {
    await api.deleteComment(id)
    comments.value = comments.value.filter((c) => c.id !== id)
  } catch (e) {
    error.value = e.message
  }
}

async function closeQuestion() {
  if (!confirm('确认关闭该问题？')) return
  try {
    await api.closeQuestion(qid.value)
    load()
  } catch (e) {
    error.value = e.message
  }
}
async function deleteQuestion() {
  if (!confirm('确认删除该问题及其全部回答？')) return
  try {
    await api.deleteQuestion(qid.value)
    router.push('/ask')
  } catch (e) {
    error.value = e.message
  }
}

watch(qid, () => {
  load()
  loadMine()
})
onMounted(() => {
  load()
  loadMine()
})
</script>

<template>
  <div v-if="loading" class="page"><p class="empty-note">加载中…</p></div>
  <div v-else-if="!q" class="page"><p class="empty-note">问题不存在或已删除</p></div>
  <div v-else class="page">
    <div class="detail-card">
      <span class="badge">{{ q.category_name }}</span>
      <span :class="['badge', q.status === 'solved' ? 'badge-green' : 'badge-gray']">
        {{ q.status === 'solved' ? '已解决' : q.status === 'closed' ? '已关闭' : '待解决' }}
      </span>
      <h2>{{ q.title }}</h2>
      <MarkdownContent :source="q.body" />
      <div class="meta" style="margin-top: 14px">
        <span v-if="q.tags">
          <span v-for="t in q.tags.split(',').filter(Boolean)" :key="t" class="tag">{{ t.trim() }}</span>
        </span>
        <span style="margin-left: auto">
          <img v-if="q.author_avatar" :src="q.author_avatar" class="avatar" alt="" />
          {{ q.author }} · {{ timeAgo(q.created_at) }} · 浏览 {{ q.views }}
        </span>
      </div>
      <div class="actions-row">
        <button class="vote-btn" :class="{ active: myVote }" @click="toggleVote('question', q.id)">
          ▲ 赞同 {{ q.score }}
        </button>
        <button class="ghost-btn" @click="toggleBookmark">{{ bookmarked ? '已收藏 ★' : '收藏 ☆' }}</button>
        <button class="ghost-btn" @click="toggleFollow">{{ following ? '已关注 ✓' : '关注问题' }}</button>
        <button v-if="q.status !== 'closed'" class="ghost-btn" @click="closeQuestion">关闭问题</button>
        <button v-if="auth.user && (auth.user.id === q.user_id || auth.isAdmin)" class="ghost-danger" @click="deleteQuestion">
          删除问题
        </button>
      </div>
    </div>

    <div class="detail-card">
      <h2 style="font-size: 18px">回答 {{ q.answer_count || answers.length }}</h2>
      <div v-if="!answers.length" class="empty-note" style="padding: 30px 0">还没有回答，来写下第一个回答吧</div>
      <div v-for="a in answers" :key="a.id" class="answer-item">
        <div class="answer-head">
          <img v-if="a.avatar" :src="a.avatar" class="avatar" alt="" />
          <span>{{ a.author }}</span>
          <span v-if="a.accepted" class="accepted-mark">已采纳 ✓</span>
          <span style="margin-left: auto">{{ timeAgo(a.created_at) }}</span>
        </div>
        <MarkdownContent v-if="editingAnswer?.id !== a.id" :source="a.body" />
        <template v-else>
          <Editor v-model="editBody" :rows="5" :maxlength="12000" />
          <div class="form-footer">
            <button class="ghost-btn" @click="editingAnswer = null">取消</button>
            <button class="primary-btn" @click="saveEditAnswer">保存</button>
          </div>
        </template>
        <div class="actions-row" v-if="editingAnswer?.id !== a.id">
          <button class="vote-btn" :class="{ active: a.voted }" @click="toggleVote('answer', a.id)">
            ▲ 赞同 {{ a.score }}
          </button>
          <button v-if="auth.user && auth.user.id === q.user_id && !a.accepted && q.status !== 'closed'"
            class="text-link" @click="acceptAnswer(a.id)">采纳</button>
          <button v-if="auth.user && (auth.user.id === a.user_id || auth.isAdmin)" class="ghost-btn"
            @click="startEditAnswer(a)">编辑</button>
          <button v-if="auth.user && (auth.user.id === a.user_id || auth.isAdmin)" class="ghost-danger"
            @click="deleteAnswer(a)">删除</button>
        </div>
      </div>
      <div v-if="auth.isLoggedIn" class="answer-box">
        <h3 style="font-size: 15px; margin-bottom: 10px">写下你的回答</h3>
        <Editor v-model="answerBody" :rows="5" :maxlength="12000" placeholder="说明推导过程、适用条件、依据或操作步骤" />
        <label style="font-size: 13px; font-weight: 600; margin-top: 12px; display: block">回答附件</label>
        <FileUpload v-model="answerFiles" />
        <p class="form-hint">{{ error }}</p>
        <div class="form-footer">
          <button class="primary-btn" @click="submitAnswer">提交回答</button>
        </div>
      </div>
      <div v-else class="empty-note" style="padding: 20px 0">
        <router-link class="text-link" to="/login">登录后参与回答</router-link>
      </div>
    </div>

    <div class="detail-card">
      <h2 style="font-size: 18px">评论 {{ comments.length }}</h2>
      <div v-for="c in comments" :key="c.id" class="answer-item" style="padding: 10px 0">
        <div class="answer-head">
          <span>{{ c.author }}</span>
          <span style="margin-left: auto">{{ timeAgo(c.created_at) }}</span>
          <button v-if="auth.user && (auth.user.id === c.user_id || auth.isAdmin)" class="ghost-danger"
            @click="deleteComment(c.id)">删除</button>
        </div>
        <p style="font-size: 14px">{{ c.body }}</p>
      </div>
      <div v-if="auth.isLoggedIn" style="display: flex; gap: 10px; margin-top: 14px">
        <input v-model="commentBody" class="search-input" style="flex: 1; min-width: 0" maxlength="500"
          placeholder="写下评论…" @keyup.enter="submitComment" />
        <button class="primary-btn" @click="submitComment">评论</button>
      </div>
    </div>
  </div>
</template>
