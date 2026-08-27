<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import api from '../api'
import { useAuthStore } from '../stores/auth'
import { timeAgo } from '../utils/time'

const router = useRouter()
const auth = useAuthStore()

const items = ref([])
const loading = ref(false)
const error = ref('')
const saving = ref(false)

const cityFilter = ref('')
const editingId = ref(0)
const newRow = ref(null)
const editForm = ref(null)

const openReviews = ref({})
const reviews = ref({}) // id -> { items, loading }
const reviewText = ref({})
const reviewAnon = ref({})

const emptyRow = () => ({
  company: '', industry: '', positions_27: '', confirm_level: '',
  strength: '', city: '', current_status: '', links: '', verified_at: '',
})

// 地点筛选按钮：从数据中提取城市 token 并按频次取前 12
const cityOptions = computed(() => {
  const counts = new Map()
  const norm = (t) => t.replace(/等$/, '').replace(/及全球基地$/, '')
  for (const it of items.value) {
    for (const t of String(it.city || '').split(/[、，,/+]/)) {
      const k = norm(t.trim())
      if (k) counts.set(k, (counts.get(k) || 0) + 1)
    }
  }
  return [...counts.entries()]
    .sort((a, b) => b[1] - a[1])
    .slice(0, 12)
    .map(([k, n]) => ({ name: k, count: n }))
})

const filteredItems = computed(() => {
  if (!cityFilter.value) return items.value
  return items.value.filter((it) => String(it.city || '').includes(cityFilter.value))
})

const linkList = (links) => String(links || '').split(/\n+/).map((s) => s.trim()).filter(Boolean)

function strengthClass(st) {
  if (st === '强') return 'strength-strong'
  if (st === '中') return 'strength-mid'
  if (st === '弱') return 'strength-weak'
  return ''
}

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

// ---- 新增 / 编辑 ----
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
  if (!newRow.value.company.trim()) return (error.value = '公司名称不能为空')
  saving.value = true
  error.value = ''
  try {
    await api.createJob({ ...newRow.value, company: newRow.value.company.trim() })
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
  editForm.value = {
    company: it.company, industry: it.industry, positions_27: it.positions_27,
    confirm_level: it.confirm_level, strength: it.strength, city: it.city,
    current_status: it.current_status, links: it.links, verified_at: it.verified_at,
  }
}
function cancelEdit() {
  editingId.value = 0
  editForm.value = null
}
async function submitEdit() {
  if (!editForm.value) return
  if (!editForm.value.company.trim()) return (error.value = '公司名称不能为空')
  saving.value = true
  error.value = ''
  try {
    await api.updateJob(editingId.value, { ...editForm.value, company: editForm.value.company.trim() })
    editingId.value = 0
    editForm.value = null
    await load()
  } catch (e) {
    error.value = e.message
  } finally {
    saving.value = false
  }
}

async function remove(it) {
  if (!requireLogin()) return
  if (!window.confirm(`确认删除「${it.company}」这一行？`)) return
  try {
    await api.deleteJob(it.id)
    await load()
  } catch (e) {
    error.value = e.message
  }
}

// ---- 公司评价 ----
async function toggleReviews(it) {
  const open = !openReviews.value[it.id]
  openReviews.value = { ...openReviews.value, [it.id]: open }
  if (open && !reviews.value[it.id]) await loadReviews(it.id)
}

async function loadReviews(id) {
  reviews.value = { ...reviews.value, [id]: { items: reviews.value[id]?.items || [], loading: true } }
  try {
    const { data } = await api.jobReviews(id)
    reviews.value = { ...reviews.value, [id]: { items: data.items, loading: false } }
  } catch (e) {
    error.value = e.message
    reviews.value = { ...reviews.value, [id]: { items: [], loading: false } }
  }
}

async function submitReview(it) {
  if (!requireLogin()) return
  const body = (reviewText.value[it.id] || '').trim()
  if (!body) return (error.value = '评价不能为空')
  saving.value = true
  error.value = ''
  try {
    await api.createJobReview(it.id, { body, anonymous: !!reviewAnon.value[it.id] })
    reviewText.value = { ...reviewText.value, [it.id]: '' }
    reviewAnon.value = { ...reviewAnon.value, [it.id]: false }
    await loadReviews(it.id)
    it.review_count = (it.review_count || 0) + 1
  } catch (e) {
    error.value = e.message
  } finally {
    saving.value = false
  }
}

async function removeReview(it, r) {
  if (!window.confirm('确认删除这条评价？')) return
  try {
    await api.deleteJobReview(r.id)
    await loadReviews(it.id)
    it.review_count = Math.max(0, (it.review_count || 0) - 1)
  } catch (e) {
    error.value = e.message
  }
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

    <div class="job-filters">
      <button class="filter-btn" :class="{ active: !cityFilter }" @click="cityFilter = ''">
        全部 <b>{{ items.length }}</b>
      </button>
      <button
        v-for="opt in cityOptions"
        :key="opt.name"
        class="filter-btn"
        :class="{ active: cityFilter === opt.name }"
        @click="cityFilter = cityFilter === opt.name ? '' : opt.name"
      >
        {{ opt.name }} <b>{{ opt.count }}</b>
      </button>
    </div>

    <p v-if="error" class="notice" style="margin-top: 14px">{{ error }}</p>
    <p v-if="!auth.isLoggedIn" class="notice" style="margin-top: 14px">
      登录后即可参与共享编辑与评价：添加公司、更新招聘信息、对公司填写评价（可匿名）。
    </p>

    <div style="overflow-x: auto; margin-top: 16px">
      <table class="admin-table job-table">
        <thead>
          <tr>
            <th>公司</th>
            <th>产业链 / 光学方向</th>
            <th>27届项目与光学岗位</th>
            <th>岗位确认度</th>
            <th>证据强度</th>
            <th>地点</th>
            <th>当前状态</th>
            <th>证据链接</th>
            <th>上次核验</th>
            <th>评价</th>
            <th>更新</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="newRow" class="job-edit-row">
            <td colspan="12">
              <div class="job-form">
                <label>公司 *<input v-model="newRow.company" maxlength="100" placeholder="公司名称" /></label>
                <label>产业链 / 光学方向<input v-model="newRow.industry" maxlength="200" /></label>
                <label>地点<input v-model="newRow.city" maxlength="100" placeholder="如：深圳、上海等" /></label>
                <label>证据强度
                  <select v-model="newRow.strength">
                    <option value="">—</option><option>强</option><option>中</option><option>弱</option>
                  </select>
                </label>
                <label>岗位确认度<input v-model="newRow.confirm_level" maxlength="300" /></label>
                <label>当前状态<input v-model="newRow.current_status" maxlength="200" /></label>
                <label>上次核验<input v-model="newRow.verified_at" type="date" /></label>
                <label class="job-form-wide">27届项目与光学岗位<textarea v-model="newRow.positions_27" maxlength="500" rows="2" /></label>
                <label class="job-form-wide">证据链接<textarea v-model="newRow.links" maxlength="1000" rows="2" placeholder="每行一个链接" /></label>
                <div class="job-form-actions">
                  <button class="filter-btn" :disabled="saving" @click="submitCreate">保存</button>
                  <button class="ghost-btn" @click="cancelCreate">取消</button>
                </div>
              </div>
            </td>
          </tr>

          <tr v-if="loading"><td colspan="12" class="job-dim" style="text-align: center">加载中…</td></tr>
          <tr v-else-if="!filteredItems.length">
            <td colspan="12" class="job-dim" style="text-align: center">
              {{ items.length ? '该地点暂无公司，试试其他筛选' : '还没有记录，点击「+ 新增一行」添加公司招聘信息' }}
            </td>
          </tr>

          <template v-for="it in filteredItems" :key="it.id">
            <tr v-if="editingId === it.id" class="job-edit-row">
              <td colspan="12">
                <div class="job-form">
                  <label>公司 *<input v-model="editForm.company" maxlength="100" /></label>
                  <label>产业链 / 光学方向<input v-model="editForm.industry" maxlength="200" /></label>
                  <label>地点<input v-model="editForm.city" maxlength="100" /></label>
                  <label>证据强度
                    <select v-model="editForm.strength">
                      <option value="">—</option><option>强</option><option>中</option><option>弱</option>
                    </select>
                  </label>
                  <label>岗位确认度<input v-model="editForm.confirm_level" maxlength="300" /></label>
                  <label>当前状态<input v-model="editForm.current_status" maxlength="200" /></label>
                  <label>上次核验<input v-model="editForm.verified_at" type="date" /></label>
                  <label class="job-form-wide">27届项目与光学岗位<textarea v-model="editForm.positions_27" maxlength="500" rows="2" /></label>
                  <label class="job-form-wide">证据链接<textarea v-model="editForm.links" maxlength="1000" rows="2" placeholder="每行一个链接" /></label>
                  <div class="job-form-actions">
                    <button class="filter-btn" :disabled="saving" @click="submitEdit">保存</button>
                    <button class="ghost-btn" @click="cancelEdit">取消</button>
                  </div>
                </div>
              </td>
            </tr>
            <tr v-else>
              <td class="job-strong">{{ it.company }}</td>
              <td class="job-cell">{{ it.industry || '—' }}</td>
              <td class="job-cell">{{ it.positions_27 || '—' }}</td>
              <td class="job-cell">{{ it.confirm_level || '—' }}</td>
              <td>
                <span v-if="it.strength" class="badge" :class="strengthClass(it.strength)">{{ it.strength }}</span>
                <span v-else class="job-dim">—</span>
              </td>
              <td>{{ it.city || '—' }}</td>
              <td class="job-cell">{{ it.current_status || '—' }}</td>
              <td>
                <template v-if="linkList(it.links).length">
                  <a
                    v-for="(l, i) in linkList(it.links).slice(0, 2)"
                    :key="i"
                    :href="l" target="_blank" rel="noopener" class="text-link job-link"
                  >链接↗</a>
                  <span v-if="linkList(it.links).length > 2" class="job-dim">+{{ linkList(it.links).length - 2 }}</span>
                </template>
                <span v-else class="job-dim">—</span>
              </td>
              <td class="job-dim">{{ it.verified_at || '—' }}</td>
              <td>
                <button class="filter-btn job-review-btn" @click="toggleReviews(it)">
                  评价 <b>{{ it.review_count || 0 }}</b>
                </button>
              </td>
              <td class="job-dim">
                <div>{{ it.updater || it.author }}</div>
                <div class="job-tiny">{{ timeAgo(it.updated_at) }}</div>
              </td>
              <td>
                <button class="filter-btn" @click="startEdit(it)">编辑</button>
                <button v-if="auth.isLoggedIn && (it.user_id === auth.user?.id || auth.isAdmin)" class="ghost-btn"
                  style="color: var(--danger)" @click="remove(it)">删除</button>
              </td>
            </tr>

            <tr v-if="openReviews[it.id]" class="job-reviews-row">
              <td colspan="12">
                <div class="job-reviews">
                  <p v-if="reviews[it.id]?.loading" class="job-dim">评价加载中…</p>
                  <p v-else-if="!reviews[it.id]?.items?.length" class="job-dim">还没有评价，来写第一条吧</p>
                  <div v-for="r in reviews[it.id]?.items" :key="r.id" class="job-review-item">
                    <div class="job-review-head">
                      <b>{{ r.author }}</b>
                      <span class="job-dim">{{ timeAgo(r.created_at) }}</span>
                      <button v-if="auth.isLoggedIn && (r.user_id === auth.user?.id || auth.isAdmin)" class="ghost-btn"
                        style="color: var(--danger)" @click="removeReview(it, r)">删除</button>
                    </div>
                    <p class="job-review-body">{{ r.body }}</p>
                  </div>
                  <div v-if="auth.isLoggedIn" class="job-review-form">
                    <textarea v-model="reviewText[it.id]" maxlength="500" rows="2" placeholder="写下你对这家公司的评价（面试体验、进度、内幕…）"></textarea>
                    <div class="job-review-actions">
                      <label class="anon-option">
                        <input v-model="reviewAnon[it.id]" type="checkbox" /> 匿名评价
                      </label>
                      <button class="filter-btn" :disabled="saving" @click="submitReview(it)">发表评价</button>
                    </div>
                  </div>
                  <p v-else class="job-dim"><a class="text-link" href="/login?redirect=/jobs">登录后即可评价</a></p>
                </div>
              </td>
            </tr>
          </template>
        </tbody>
      </table>
    </div>
  </div>
</template>

<style scoped>
.job-filters {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  margin-top: 4px;
}
.job-filters .filter-btn.active {
  background: var(--primary);
  border-color: var(--primary);
  color: #fff;
}
.job-filters .filter-btn b {
  color: var(--teal);
  margin-left: 2px;
}
.job-filters .filter-btn.active b {
  color: #fff;
}
.job-table .job-strong {
  font-weight: 600;
  white-space: nowrap;
}
.job-table .job-cell {
  max-width: 260px;
}
.job-table .job-dim {
  color: var(--muted);
  font-size: 12px;
}
.job-table .job-tiny {
  font-size: 11px;
}
.job-table .job-link {
  display: inline-block;
  margin-right: 6px;
  font-size: 12px;
}
.strength-strong {
  background: #e3f4e9;
  color: #2e8b57;
}
.strength-mid {
  background: #fdf3e0;
  color: #b8860b;
}
.strength-weak {
  background: #eef0f2;
  color: #7a8288;
}
.job-edit-row {
  background: var(--blue-soft);
}
.job-edit-row input,
.job-edit-row select,
.job-edit-row textarea {
  border-color: var(--blue);
}
.job-form {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 10px 14px;
}
.job-form label {
  display: flex;
  flex-direction: column;
  gap: 4px;
  font-size: 12px;
  color: var(--muted);
}
.job-form input,
.job-form select,
.job-form textarea {
  border: 1px solid var(--line);
  border-radius: 5px;
  padding: 6px 8px;
  font-size: 13px;
  font-family: inherit;
  color: var(--ink);
}
.job-form .job-form-wide {
  grid-column: span 2;
}
.job-form-actions {
  display: flex;
  align-items: flex-end;
  gap: 8px;
}
.job-reviews-row {
  background: #f7fafd;
}
.job-reviews {
  padding: 4px 8px;
}
.job-review-item {
  border-bottom: 1px dashed var(--line);
  padding: 8px 0;
}
.job-review-item:last-of-type {
  border-bottom: 0;
}
.job-review-head {
  display: flex;
  align-items: center;
  gap: 8px;
}
.job-review-head .ghost-btn {
  margin-left: auto;
  padding: 0;
  font-size: 12px;
}
.job-review-body {
  margin: 6px 0 0;
  font-size: 14px;
  white-space: pre-wrap;
}
.job-review-form {
  margin-top: 10px;
}
.job-review-form textarea {
  width: 100%;
  max-width: 640px;
  border: 1px solid var(--line);
  border-radius: 6px;
  padding: 8px;
  font-size: 14px;
  font-family: inherit;
  resize: vertical;
}
.job-review-actions {
  display: flex;
  align-items: center;
  gap: 14px;
  margin-top: 8px;
}
.job-review-actions .anon-option {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  color: var(--muted);
  cursor: pointer;
}
.job-review-actions .anon-option input {
  width: 15px;
  height: 15px;
  accent-color: var(--primary);
}
</style>
