<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import api from '../api'
import MarkdownContent from '../components/MarkdownContent.vue'
import Editor from '../components/Editor.vue'
import { timeAgo } from '../utils/time'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const id = computed(() => Number(route.params.id))

const post = ref(null)
const myVote = ref(false)
const replyBody = ref('')
const error = ref('')
const loading = ref(true)
const editingReply = ref(null)
const editBody = ref('')
const replyEditor = ref(null)

function goReply() {
  if (!auth.isLoggedIn) return router.push('/login?redirect=' + route.path)
  document.getElementById('reply-box')?.scrollIntoView({ behavior: 'smooth', block: 'start' })
  setTimeout(() => replyEditor.value?.focus(), 350)
}

async function load() {
  loading.value = true
  try {
    const { data } = await api.forumPost(id.value)
    post.value = data
    api.viewForumPost(id.value).catch(() => {})
  } finally {
    loading.value = false
  }
}

async function toggleVote(targetType, targetId) {
  if (!auth.isLoggedIn) return router.push('/login')
  try {
    const { data } = await api.toggleVote({ target_type: targetType, target_id: targetId })
    if (targetType === 'forum_post') {
      myVote.value = data.voted
      post.value.score += data.voted ? 1 : -1
    } else {
      const r = post.value.replies.find((x) => x.id === targetId)
      if (r) {
        r.voted = data.voted
        r.score += data.voted ? 1 : -1
      }
    }
  } catch (e) {
    error.value = e.message
  }
}

async function submitReply() {
  error.value = ''
  if (!replyBody.value.trim()) return (error.value = '回复内容不能为空')
  try {
    await api.createForumReply(id.value, { body: replyBody.value })
    replyBody.value = ''
    load()
  } catch (e) {
    error.value = e.message
  }
}

function startEditReply(r) {
  editingReply.value = r
  editBody.value = r.body
}
async function saveEditReply() {
  try {
    await api.updateForumReply(editingReply.value.id, { body: editBody.value })
    editingReply.value = null
    load()
  } catch (e) {
    error.value = e.message
  }
}
async function deleteReply(r) {
  if (!confirm('确认删除该回复？')) return
  try {
    await api.deleteForumReply(r.id)
    load()
  } catch (e) {
    error.value = e.message
  }
}

async function deletePost() {
  if (!confirm('确认删除该帖子及其全部回复？')) return
  try {
    await api.deleteForumPost(id.value)
    router.push('/forum')
  } catch (e) {
    error.value = e.message
  }
}

async function togglePin() {
  try {
    await api.pinForumPost(id.value, { pinned: !post.value.is_pinned })
    post.value.is_pinned = !post.value.is_pinned
  } catch (e) {
    error.value = e.message
  }
}

async function toggleSolved() {
  const cur = post.value.is_solved
  try {
    await api.updateForumPost(id.value, {
      board_id: post.value.board_id,
      title: post.value.title,
      body: post.value.body,
      tags: post.value.tags || '',
      is_solved: !cur,
    })
    post.value.is_solved = !cur
  } catch (e) {
    error.value = e.message
  }
}

onMounted(async () => {
  load()
  if (auth.isLoggedIn) {
    try {
      const { data } = await api.voteStatus({ target_type: 'forum_post', target_id: id.value })
      myVote.value = data.voted
    } catch { /* 忽略 */ }
  }
})
</script>

<template>
  <div v-if="loading" class="page"><p class="empty-note">加载中…</p></div>
  <div v-else-if="!post" class="page"><p class="empty-note">帖子不存在或已删除</p></div>
  <div v-else class="page page-narrow">
    <div class="detail-card">
      <span class="badge badge-teal">{{ post.board_name }}</span>
      <span v-if="post.is_pinned" class="badge badge-pinned">置顶</span>
      <span v-if="post.is_solved" class="badge badge-green">已解决</span>
      <h2>{{ post.title }}</h2>
      <div v-if="post.tags" class="meta" style="margin-top: 8px">
        <span v-for="t in post.tags.split(',').filter(Boolean)" :key="t" class="tag">{{ t.trim() }}</span>
      </div>
      <MarkdownContent :source="post.body" />
      <div class="meta" style="margin-top: 14px">
        <span style="margin-left: auto">
          <img v-if="post.author_avatar" :src="post.author_avatar" class="avatar" alt="" />
          {{ post.author }} · {{ timeAgo(post.created_at) }} · 浏览 {{ post.views }}
        </span>
      </div>
      <div class="actions-row">
        <button class="primary-btn" style="padding: 7px 16px" @click="goReply">参与回复</button>
        <button class="vote-btn" :class="{ active: myVote }" @click="toggleVote('forum_post', post.id)">
          ▲ 赞 {{ post.score }}
        </button>
        <button v-if="auth.isAdmin" class="ghost-btn" @click="togglePin">
          {{ post.is_pinned ? '取消置顶' : '置顶' }}
        </button>
        <button v-if="auth.user && (auth.user.id === post.user_id || auth.isAdmin)" class="ghost-btn" @click="toggleSolved">
          {{ post.is_solved ? '标记未解决' : '标记已解决' }}
        </button>
        <button v-if="auth.user && (auth.user.id === post.user_id || auth.isAdmin)" class="ghost-danger"
          @click="deletePost">删除帖子</button>
      </div>
    </div>

    <div class="detail-card">
      <h2 style="font-size: 18px">回复 {{ post.reply_count || post.replies.length }}</h2>
      <div v-if="!post.replies.length" class="empty-note" style="padding: 30px 0">还没有回复，来抢沙发</div>
      <div v-for="r in post.replies" :key="r.id" class="answer-item">
        <div class="answer-head">
          <img v-if="r.avatar" :src="r.avatar" class="avatar" alt="" />
          <span>{{ r.author }}</span>
          <span style="margin-left: auto">{{ timeAgo(r.created_at) }}</span>
        </div>
        <MarkdownContent v-if="editingReply?.id !== r.id" :source="r.body" />
        <template v-else>
          <Editor v-model="editBody" :rows="4" :maxlength="12000" />
          <div class="form-footer">
            <button class="ghost-btn" @click="editingReply = null">取消</button>
            <button class="primary-btn" @click="saveEditReply">保存</button>
          </div>
        </template>
        <div class="actions-row" v-if="editingReply?.id !== r.id">
          <button class="vote-btn" :class="{ active: r.voted }" @click="toggleVote('forum_reply', r.id)">
            ▲ 赞 {{ r.score }}
          </button>
          <button v-if="auth.user && (auth.user.id === r.user_id || auth.isAdmin)" class="ghost-btn"
            @click="startEditReply(r)">编辑</button>
          <button v-if="auth.user && (auth.user.id === r.user_id || auth.isAdmin)" class="ghost-danger"
            @click="deleteReply(r)">删除</button>
        </div>
      </div>
      <div v-if="auth.isLoggedIn" id="reply-box" class="answer-box">
        <h3 style="font-size: 15px; margin-bottom: 10px">回复帖子</h3>
        <Editor ref="replyEditor" v-model="replyBody" :rows="4" :maxlength="12000" />
        <p class="form-hint">{{ error }}</p>
        <div class="form-footer">
          <button class="primary-btn" @click="submitReply">发表回复</button>
        </div>
      </div>
      <div v-else class="empty-note" style="padding: 20px 0">
        <router-link class="text-link" to="/login">登录后参与回复</router-link>
      </div>
    </div>
  </div>
</template>

<style scoped>
.badge-pinned {
  background: #ffe9d6;
  color: #d2691e;
}
</style>
