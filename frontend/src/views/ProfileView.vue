<script setup>
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import api from '../api'

const router = useRouter()
const auth = useAuthStore()
const profile = ref({ username: '', bio: '', expertise: '' })
const notifications = ref([])
const unread = ref(0)
const error = ref('')
const hint = ref('')
const stats = ref({ following: 0, followers: 0, received_likes: 0, posts: 0, comments: 0 })
const oldPassword = ref('')
const newPassword = ref('')
const passwordHint = ref('')

onMounted(async () => {
  if (!auth.isLoggedIn) return router.push('/login?redirect=/profile')
  try {
    const { data } = await api.me()
    profile.value = { username: data.username, bio: data.bio, expertise: data.expertise }
    stats.value = data.stats || { following: 0, followers: 0, received_likes: 0, posts: 0, comments: 0 }
    const { data: nd } = await api.notifications()
    notifications.value = nd.items
    unread.value = nd.unread
  } catch (e) {
    error.value = e.message
  }
})

async function save() {
  hint.value = ''
  try {
    await api.updateMe(profile.value)
    auth.fetchMe()
    hint.value = '资料已保存'
  } catch (e) {
    error.value = e.message
  }
}

async function changePassword() {
  passwordHint.value = ''
  if (newPassword.value.length < 8) return (passwordHint.value = '新密码至少 8 位')
  try {
    await api.changePassword({ old_password: oldPassword.value, new_password: newPassword.value })
    oldPassword.value = ''
    newPassword.value = ''
    passwordHint.value = '密码已修改，其他设备将需要重新登录'
  } catch (e) {
    passwordHint.value = e.message
  }
}

async function readAll() {
  try {
    await api.readNotifications()
    unread.value = 0
    notifications.value.forEach((n) => (n.read = true))
  } catch { /* 忽略 */ }
}

async function logout() {
  await auth.logout()
  router.push('/')
}
</script>

<template>
  <div class="page">
    <div class="section-heading">
      <div>
        <p class="eyebrow">ACCOUNT</p>
        <h2>账号设置</h2>
      </div>
    </div>
    <div class="layout">
      <div>
        <div class="stat-grid" style="grid-template-columns: repeat(5, 1fr)">
          <div class="stat-cell">
            <strong>{{ stats.following }}</strong>
            <span>关注了</span>
          </div>
          <div class="stat-cell">
            <strong>{{ stats.followers }}</strong>
            <span>关注者</span>
          </div>
          <div class="stat-cell">
            <strong>{{ stats.received_likes }}</strong>
            <span>获得赞</span>
          </div>
          <div class="stat-cell">
            <strong>{{ stats.posts }}</strong>
            <span>发帖</span>
          </div>
          <div class="stat-cell">
            <strong>{{ stats.comments }}</strong>
            <span>评论</span>
          </div>
        </div>
        <div class="form-card">
          <label>显示名称</label>
          <input v-model="profile.username" type="text" maxlength="30" />
          <label>专业简介</label>
          <textarea v-model="profile.bio" rows="3" maxlength="160"
            placeholder="例如：成像光学方向硕士生，关注镜头设计与公差分析"></textarea>
          <label>擅长方向</label>
          <input v-model="profile.expertise" type="text" maxlength="200"
            placeholder="用逗号分隔，例如：成像光学, Zemax, MTF" />
          <p class="form-hint">{{ error || hint }}</p>
          <div class="form-footer">
            <button class="ghost-btn danger" style="color: var(--danger)" @click="logout">退出登录</button>
            <button class="primary-btn" @click="save">保存资料</button>
          </div>
        </div>
        <div class="form-card" style="margin-top: 18px">
          <h2 style="font-size: 18px">修改密码</h2>
          <label>原密码</label>
          <input v-model="oldPassword" type="password" autocomplete="current-password" />
          <label>新密码</label>
          <input v-model="newPassword" type="password" autocomplete="new-password" minlength="8" maxlength="72"
            placeholder="至少 8 位" />
          <p class="form-hint">{{ passwordHint }}</p>
          <div class="form-footer">
            <button class="filter-btn" @click="changePassword">修改密码</button>
          </div>
        </div>
      </div>
      <aside>
        <div class="side-panel">
          <div class="side-title">通知（{{ unread }} 未读）</div>
          <button class="text-link" style="font-size: 12px; margin-bottom: 8px" @click="readAll">全部标记已读</button>
          <div v-if="!notifications.length" style="color: var(--muted); font-size: 12px">暂无通知</div>
          <div v-for="n in notifications" :key="n.id"
            style="padding: 8px 0; border-bottom: 1px solid var(--line); font-size: 13px">
            <router-link v-if="n.question_id" :to="`/ask/${n.question_id}`">{{ n.message }}</router-link>
            <span v-else>{{ n.message }}</span>
            <div style="color: var(--muted); font-size: 11px; margin-top: 2px">{{ n.read ? '已读' : '● 未读' }}</div>
          </div>
        </div>
      </aside>
    </div>
  </div>
</template>
