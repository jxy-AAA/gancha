<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import api from '../api'
import { useAuthStore } from '../stores/auth'
import { timeAgo } from '../utils/time'

const router = useRouter()
const auth = useAuthStore()

const STATUS = {
  pending: '未投递',
  applied: '已投递',
  test: '笔试',
  interview: '面试',
  offer: 'Offer',
  rejected: '已拒',
}

const items = ref([])
const loading = ref(false)
const error = ref('')
const editingId = ref(0)
const newRow = ref(null)
const saving = ref(false)

const emptyRow = () => ({ company: '', position: '', city: '', status: 'pending', url: '', note: '' })

const stats = computed(() => {
  const s = { total: items.value.length }
  for (const k of Object.keys(STATUS)) s[k] = 0
  for (const it of items.value) if (STATUS[it.status]) s[it.status]++
  return s
})

async function load() {
  loading.value = true
  try {
    const { data } = await api.jobs()
    items.value = data.items
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}

function requireLogin() {
  if (!auth.isLoggedIn) {
    router.push('/login?redirect=/jobs')
    return false
  }
  return true
}

function startCreate() {
  if (!requireLogin()) return
  newRow.value = emptyRow()
  editingId.value = 0
}
function cancelCreate() {
  newRow.value = null
}
async function submitCreate() {
  if (!newRow.value) return
  const row = newRow.value
  if (!row.company.trim() || !row.position.trim()) {
    return (error.value = '公司和岗位不能为空')
  }
  saving.value = true
  error.value = ''
  try {
    await api.createJob({ ...row, company: row.company.trim(), position: row.position.trim() })
    newRow.value = null
    await load()
  } catch (e) {
    error.value = e.message
  } finally {
    saving.value = false
  }
}

function startEdit(it) {
  if (!requireLogin()) return
  editingId.value = it.id
  it._edit = { company: it.company, position: it.position, city: it.city, status: it.status, url: it.url, note: it.note }
}
function cancelEdit(it) {
  editingId.value = 0
  delete it._edit
}
async function submitEdit(it) {
  if (!it._edit) return
  if (!it._edit.company.trim() || !it._edit.position.trim()) {
    return (error.value = '公司和岗位不能为空')
  }
  saving.value = true
  error.value = ''
  try {
    await api.updateJob(it.id, { ...it._edit, company: it._edit.company.trim(), position: it._edit.position.trim() })
    editingId.value = 0
    await load()
  } catch (e) {
    error.value = e.message
  } finally {
    saving.value = false
  }
}

async function remove(it) {
  if (!requireLogin()) return
  if (!window.confirm(`确认删除「${it.company} · ${it.position}」这一行？`)) return
  try {
    await api.deleteJob(it.id)
    await load()
  } catch (e) {
    error.value = e.message
  }
}

function statusClass(st) {
  if (st === 'offer') return 'badge-green'
  if (st === 'rejected') return 'badge-gray'
  return 'badge-teal'
}

onMounted(load)
</script>

<template>
  <div class="page">
    <div class="section-heading">
      <div>
        <p class="eyebrow">JOBS</p>
        <h2>就业信息</h2>
      </div>
      <div class="list-tools">
        <button class="primary-btn" @click="startCreate">+ 新增一行</button>
      </div>
    </div>

    <div class="job-stats">
      <span class="job-stat"><b>{{ stats.total }}</b> 总记录</span>
      <span v-for="(label, k) in STATUS" :key="k" class="job-stat" :class="{ 'job-stat-offer': k === 'offer' }">
        <b>{{ stats[k] }}</b> {{ label }}
      </span>
    </div>

    <p v-if="error" class="notice" style="margin-top: 14px">{{ error }}</p>
    <p v-if="!auth.isLoggedIn" class="notice" style="margin-top: 14px">
      登录后即可参与共享编辑：添加岗位、更新投递进度。点击「新增一行」或行内「编辑」将跳转登录。
    </p>

    <div style="overflow-x: auto; margin-top: 16px">
      <table class="admin-table job-table">
        <thead>
          <tr>
            <th>公司</th>
            <th>岗位</th>
            <th>城市</th>
            <th>状态</th>
            <th>投递链接</th>
            <th>备注</th>
            <th>更新人</th>
            <th>更新时间</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="newRow" class="job-edit-row">
            <td><input v-model="newRow.company" maxlength="100" placeholder="公司" /></td>
            <td><input v-model="newRow.position" maxlength="100" placeholder="岗位" /></td>
            <td><input v-model="newRow.city" maxlength="50" placeholder="城市" /></td>
            <td>
              <select v-model="newRow.status">
                <option v-for="(label, k) in STATUS" :key="k" :value="k">{{ label }}</option>
              </select>
            </td>
            <td><input v-model="newRow.url" maxlength="500" placeholder="https://…" /></td>
            <td><input v-model="newRow.note" maxlength="500" placeholder="备注（内推码、进度等）" /></td>
            <td class="job-dim">{{ auth.user?.username || '—' }}</td>
            <td class="job-dim">刚刚</td>
            <td>
              <button class="filter-btn" :disabled="saving" @click="submitCreate">保存</button>
              <button class="ghost-btn" @click="cancelCreate">取消</button>
            </td>
          </tr>

          <tr v-if="loading"><td colspan="9" class="job-dim" style="text-align: center">加载中…</td></tr>
          <tr v-else-if="!items.length"><td colspan="9" class="job-dim" style="text-align: center">还没有记录，点击「+ 新增一行」开始共享秋招信息吧</td></tr>

          <tr v-for="it in items" :key="it.id">
            <template v-if="editingId === it.id">
              <td><input v-model="it._edit.company" maxlength="100" /></td>
              <td><input v-model="it._edit.position" maxlength="100" /></td>
              <td><input v-model="it._edit.city" maxlength="50" /></td>
              <td>
                <select v-model="it._edit.status">
                  <option v-for="(label, k) in STATUS" :key="k" :value="k">{{ label }}</option>
                </select>
              </td>
              <td><input v-model="it._edit.url" maxlength="500" /></td>
              <td><input v-model="it._edit.note" maxlength="500" /></td>
              <td class="job-dim">{{ it.updater || it.author }}</td>
              <td class="job-dim">{{ timeAgo(it.updated_at) }}</td>
              <td>
                <button class="filter-btn" :disabled="saving" @click="submitEdit(it)">保存</button>
                <button class="ghost-btn" @click="cancelEdit(it)">取消</button>
              </td>
            </template>
            <template v-else>
              <td class="job-strong">{{ it.company }}</td>
              <td>{{ it.position }}</td>
              <td>{{ it.city || '—' }}</td>
              <td><span class="badge" :class="statusClass(it.status)">{{ STATUS[it.status] || it.status }}</span></td>
              <td>
                <a v-if="it.url" :href="it.url" target="_blank" rel="noopener" class="text-link">链接 ↗</a>
                <span v-else class="job-dim">—</span>
              </td>
              <td class="job-note">{{ it.note || '—' }}</td>
              <td class="job-dim">{{ it.updater || it.author }}</td>
              <td class="job-dim">{{ timeAgo(it.updated_at) }}</td>
              <td>
                <button class="filter-btn" @click="startEdit(it)">编辑</button>
                <button v-if="auth.isLoggedIn && (it.user_id === auth.user?.id || auth.isAdmin)" class="ghost-btn"
                  style="color: var(--danger)" @click="remove(it)">删除</button>
              </td>
            </template>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<style scoped>
.job-stats {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
}
.job-stat {
  background: #fff;
  border: 1px solid var(--line);
  border-radius: 8px;
  padding: 10px 16px;
  font-size: 13px;
  color: var(--muted);
}
.job-stat b {
  color: var(--ink);
  font-size: 17px;
  margin-right: 4px;
}
.job-stat-offer b {
  color: #3d9a5f;
}
.job-table input,
.job-table select {
  width: 100%;
  min-width: 90px;
  border: 1px solid var(--line);
  border-radius: 5px;
  padding: 6px 8px;
  font-size: 13px;
  font-family: inherit;
}
.job-table select {
  min-width: 84px;
}
.job-table .job-strong {
  font-weight: 600;
}
.job-table .job-note {
  max-width: 220px;
}
.job-table .job-dim {
  color: var(--muted);
  font-size: 12px;
}
.job-edit-row {
  background: var(--blue-soft);
}
.job-edit-row input,
.job-edit-row select {
  border-color: var(--blue);
}
</style>
