<script setup>
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../../stores/auth'
import api from '../../api'
import Pagination from '../../components/Pagination.vue'
import { formatDate } from '../../utils/time'

const router = useRouter()
const auth = useAuthStore()
const tab = ref('stats')
const stats = ref(null)
const users = ref([])
const userTotal = ref(0)
const userPage = ref(1)
const userSearch = ref('')
const categories = ref([])
const audit = ref([])
const error = ref('')

const roleLabel = { new_user: '新用户', user: '普通用户', admin: '管理员' }

onMounted(async () => {
  if (!auth.isLoggedIn) return router.push('/login?redirect=/admin')
  if (!auth.isAdmin) {
    error.value = '需要管理员权限'
    return
  }
  await loadStats()
  await loadUsers()
  await loadCategories()
  await loadAudit()
})

async function loadStats() {
  const { data } = await api.adminStats()
  stats.value = data
}
async function loadUsers() {
  const { data } = await api.adminUsers({ page: userPage.value, page_size: 10, search: userSearch.value || undefined })
  users.value = data.items
  userTotal.value = data.total
}
async function loadCategories() {
  const { data } = await api.categories()
  categories.value = data.items
}
async function loadAudit() {
  const { data } = await api.adminAudit({ page: 1, page_size: 20 })
  audit.value = data.items
}
function onUserPage(p) {
  userPage.value = p
  loadUsers()
}
function searchUsers() {
  userPage.value = 1
  loadUsers()
}

async function setRole(u, role) {
  try {
    await api.updateAdminUser(u.id, { role })
    u.role = role
    await loadAudit()
  } catch (e) {
    alert(e.message)
  }
}
async function toggleSuspend(u) {
  const until = u.suspended_until ? '' : new Date(Date.now() + 7 * 86400000).toISOString()
  if (!confirm(u.suspended_until ? '解封该用户？' : '暂停该用户 7 天？')) return
  try {
    await api.updateAdminUser(u.id, { role: u.role, suspended_until: until })
    u.suspended_until = until
  } catch (e) {
    alert(e.message)
  }
}
async function removeUser(u) {
  if (!confirm(`确认删除用户 ${u.username} 及其全部内容？此操作不可恢复`)) return
  try {
    await api.deleteAdminUser(u.id)
    await loadUsers()
  } catch (e) {
    alert(e.message)
  }
}

async function addCategory() {
  const name = prompt('新分类名称：')
  if (!name) return
  try {
    await api.createCategory({ name, active: true, position: categories.value.length })
    await loadCategories()
  } catch (e) {
    alert(e.message)
  }
}
async function toggleCategory(c) {
  try {
    await api.updateCategory(c.id, { name: c.name, active: !c.active, position: c.position })
    c.active = !c.active
  } catch (e) {
    alert(e.message)
  }
}
async function removeCategory(c) {
  if (!confirm(`确认删除分类「${c.name}」？`)) return
  try {
    await api.deleteCategory(c.id)
    await loadCategories()
  } catch (e) {
    alert(e.message)
  }
}
</script>

<template>
  <div class="page">
    <div class="section-heading">
      <div>
        <p class="eyebrow">ADMIN</p>
        <h2>管理后台</h2>
      </div>
      <div class="list-tools">
        <button class="filter-btn" :class="{ active: tab === 'stats' }" @click="tab = 'stats'">统计</button>
        <button class="filter-btn" :class="{ active: tab === 'users' }" @click="tab = 'users'">用户</button>
        <button class="filter-btn" :class="{ active: tab === 'categories' }" @click="tab = 'categories'">分类</button>
        <button class="filter-btn" :class="{ active: tab === 'audit' }" @click="tab = 'audit'">审计</button>
      </div>
    </div>

    <p v-if="error" class="notice">{{ error }}</p>

    <template v-if="tab === 'stats' && stats">
      <div class="stat-grid">
        <div class="stat-cell"><strong>{{ stats.users }}</strong><span>用户</span></div>
        <div class="stat-cell"><strong>{{ stats.questions }}</strong><span>问题</span></div>
        <div class="stat-cell"><strong>{{ stats.answers }}</strong><span>回答</span></div>
        <div class="stat-cell"><strong>{{ stats.articles }}</strong><span>知识文章</span></div>
        <div class="stat-cell"><strong>{{ stats.forum_posts }}</strong><span>论坛帖子</span></div>
        <div class="stat-cell"><strong>{{ stats.forum_replies }}</strong><span>论坛回复</span></div>
        <div class="stat-cell"><strong>{{ stats.uploads }}</strong><span>附件</span></div>
        <div class="stat-cell"><strong>{{ stats.notifications }}</strong><span>通知</span></div>
      </div>
    </template>

    <template v-if="tab === 'users'">
      <div class="list-tools" style="margin-bottom: 14px">
        <input v-model="userSearch" class="search-input" type="search" placeholder="搜索用户名或邮箱" @keyup.enter="searchUsers" />
        <button class="filter-btn" @click="searchUsers">搜索</button>
      </div>
      <table class="admin-table">
        <thead>
          <tr><th>ID</th><th>用户名</th><th>邮箱</th><th>角色</th><th>注册时间</th><th>状态</th><th>操作</th></tr>
        </thead>
        <tbody>
          <tr v-for="u in users" :key="u.id">
            <td>{{ u.id }}</td>
            <td>{{ u.username }}</td>
            <td>{{ u.email }}</td>
            <td>
              <select :value="u.role" @change="setRole(u, $event.target.value)">
                <option v-for="(label, key) in roleLabel" :key="key" :value="key">{{ label }}</option>
              </select>
            </td>
            <td>{{ formatDate(u.created_at) }}</td>
            <td>{{ u.suspended_until ? '已暂停' : '正常' }}</td>
            <td>
              <button class="text-link" style="margin-right: 10px" @click="toggleSuspend(u)">
                {{ u.suspended_until ? '解封' : '暂停' }}
              </button>
              <button class="ghost-danger" @click="removeUser(u)">删除</button>
            </td>
          </tr>
        </tbody>
      </table>
      <Pagination :page="userPage" :total="userTotal" :page-size="10" @change="onUserPage" />
    </template>

    <template v-if="tab === 'categories'">
      <button class="primary-btn" style="margin-bottom: 14px" @click="addCategory">+ 新增分类</button>
      <table class="admin-table">
        <thead>
          <tr><th>ID</th><th>名称</th><th>启用</th><th>排序</th><th>操作</th></tr>
        </thead>
        <tbody>
          <tr v-for="c in categories" :key="c.id">
            <td>{{ c.id }}</td>
            <td>{{ c.name }}</td>
            <td>{{ c.active ? '是' : '否' }}</td>
            <td>{{ c.position }}</td>
            <td>
              <button class="text-link" style="margin-right: 10px" @click="toggleCategory(c)">
                {{ c.active ? '停用' : '启用' }}
              </button>
              <button class="ghost-danger" @click="removeCategory(c)">删除</button>
            </td>
          </tr>
        </tbody>
      </table>
    </template>

    <template v-if="tab === 'audit'">
      <table class="admin-table">
        <thead>
          <tr><th>时间</th><th>操作者</th><th>动作</th><th>目标</th><th>详情</th></tr>
        </thead>
        <tbody>
          <tr v-for="a in audit" :key="a.id">
            <td>{{ formatDate(a.created_at) }}</td>
            <td>{{ a.actor_name || a.actor_id }}</td>
            <td>{{ a.action }}</td>
            <td>{{ a.target_type }}#{{ a.target_id || '-' }}</td>
            <td>{{ a.details }}</td>
          </tr>
        </tbody>
      </table>
    </template>
  </div>
</template>
