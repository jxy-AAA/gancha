<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import api from '../api'
import { timeAgo } from '../utils/time'

const props = defineProps({
  q: { type: Object, required: true },
  showBody: { type: Boolean, default: true },
})

const router = useRouter()
const auth = useAuthStore()
const voted = ref(props.q.voted || false)
const score = ref(props.q.score || 0)
const busy = ref(false)

const statusLabel = { open: '待解决', solved: '已解决', closed: '已关闭' }
const statusClass = { open: 'badge-gray', solved: 'badge-green', closed: 'badge-gray' }

async function toggleVote() {
  if (!auth.isLoggedIn) return router.push('/login?redirect=/ask')
  if (busy.value) return
  busy.value = true
  try {
    const { data } = await api.toggleVote({ target_type: 'question', target_id: props.q.id })
    voted.value = data.voted
    score.value += data.voted ? 1 : -1
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="card">
    <h3>
      <router-link :to="`/ask/${q.id}`">{{ q.title }}</router-link>
    </h3>
    <p v-if="showBody && q.body" class="excerpt">{{ q.body.replace(/[#*`$>\[\]|]/g, '').slice(0, 160) }}</p>
    <div class="meta">
      <span class="badge">{{ q.category_name }}</span>
      <span :class="['badge', statusClass[q.status]]">{{ statusLabel[q.status] }}</span>
      <button class="like-btn" :class="{ active: voted }" :disabled="busy" @click.stop="toggleVote">
        ▲ {{ score }}
      </button>
      <span>💬 {{ q.comment_count ?? 0 }}</span>
      <span>回答 {{ q.answer_count }}</span>
      <span>浏览 {{ q.views }}</span>
      <span v-if="q.tags">
        <span v-for="t in q.tags.split(',').filter(Boolean).slice(0, 4)" :key="t" class="tag">{{ t.trim() }}</span>
      </span>
      <span style="margin-left: auto">
        <img v-if="q.author_avatar" :src="q.author_avatar" class="avatar" alt="" />
        {{ q.author }} · {{ timeAgo(q.created_at) }}
      </span>
    </div>
  </div>
</template>
