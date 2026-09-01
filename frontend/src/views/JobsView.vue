<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import api from '../api'
import { useAuthStore } from '../stores/auth'
import { timeAgo, formatDate } from '../utils/time'

const router = useRouter()
const auth = useAuthStore()

const items = ref([])
const loading = ref(false)
const error = ref('')
const saving = ref(false)

// 筛选（服务端筛选）
const statusFilter = ref('active') // active | invalid | duplicate | all
const cityKeyword = ref('')
const industryKeyword = ref('')

// 每张卡片的操作面板：panel.id + panel.mode(edit/flag/versions/reviews)
const panel = ref({ id: 0, mode: '' })
const editForm = ref(null)
const flagForm = ref({ flag: 'invalid', reason: '' })
const versions = ref([])
const versionsLoading = ref(false)
const reviews = ref({}) // id -> { items, loading }
const reviewText = ref({})
const reviewAnon = ref({})

const statusLabel = { active: '正常', invalid: '已失效', duplicate: '重复', all: '全部' }
const statusClass = { active: 'badge-green', invalid: 'badge-gray', duplicate: 'badge-teal' }

const cityOptions = computed(() => {
  const set = new Set()
  const norm = (t) => t.replace(/等$/, '').replace(/及全球基地$/, '')
  for (const it of items.value) {
    for (const t of String(it.city || '').split(/[、，,/+]/)) {
      const k = norm(t.trim())
      if (k) set.add(k)
    }
  }
  return [...set].sort()
})

const filteredItems = computed(() => {
  let list = items.value
  if (statusFilter.value !== 'all') list = list.filter((it) => it.status === statusFilter.value)
  return list
})

const statusCounts = computed(() => {
  const c = { active: 0, invalid: 0, duplicate: 0, all: items.value.length }
  for (const it of items.value) c[it.status] = (c[it.status] || 0) + 1
  return c
})

const linkList = (links) => String(links || '').split(/\n+/).map((s) => s.trim()).filter(Boolean)

// 核验过期提示：核验时间超过 30 天提示可能过期
function verifyHint(it) {
  if (!it.verified_at) return null
  const v = new Date(String(it.verified_at).replace(/-/g, '/'))
  if (isNaN(v.getTime())) return null
  const days = Math.floor((Date.now() - v.getTime()) / 86400000)
  if (days > 30) return { cls: 'job-expired', text: `⚠ 已 ${days} 天未核验，可能过期` }
  return { cls: 'job-fresh', text: `${days} 天前核验` }
}

async function load() {
  loading.value = true
  try {
    const { data } = await api.jobs({
      status: statusFilter.value,
      city: cityKeyword.value || undefined,
      industry: industryKeyword.value || undefined,
    })
    items.value = data.items
    error.value = ''
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}

function applyFilter() {
  panel.value = { id: 0, mode: '' }
  load()
}

function requireLogin() {
  if (!auth.isLoggedIn) {
    router.push('/login?redirect=/jobs')
    return false
  }
  return true
}

// ---- 新增 ----
const newForm = ref(null)
function startCreate() {
  if (!requireLogin()) return
  newForm.value = {
    company: '', industry: '', city: '', apply_link: '',
    referral_code: '', verified_at: '', edit_reason: '',
  }
  panel.value = { id: 0, mode: '' }
}
function cancelCreate() {
  newForm.value = null
}
async function submitCreate() {
  if (!newForm.value) return
  if (!newForm.value.company.trim()) return (error.value = '公司名称不能为空')
  saving.value = true
  error.value = ''
  try {
    await api.createJob({ ...newForm.value, company: newForm.value.company.trim() })
    newForm.value = null
    await load()
  } catch (e) {
    error.value = e.message
  } finally {
    saving.value = false
  }
}

// ---- 编辑（必须填写修改原因）----
function startEdit(it) {
  if (!requireLogin()) return
  panel.value = { id: it.id, mode: 'edit' }
  editForm.value = {
    company: it.company, industry: it.industry, city: it.city,
    apply_link: it.apply_link, referral_code: it.referral_code, verified_at: it.verified_at,
    edit_reason: '',
  }
}
function cancelEdit() {
  panel.value = { id: 0, mode: '' }
  editForm.value = null
}
async function submitEdit() {
  if (!editForm.value) return
  if (!editForm.value.company.trim()) return (error.value = '公司名称不能为空')
  if (!editForm.value.edit_reason.trim()) return (error.value = '修改原因不能为空')
  saving.value = true
  error.value = ''
  try {
    await api.updateJob(panel.value.id, { ...editForm.value, company: editForm.value.company.trim() })
    panel.value = { id: 0, mode: '' }
    editForm.value = null
    await load()
  } catch (e) {
    error.value = e.message
  } finally {
    saving.value = false
  }
}

// ---- 标记失效 / 重复（替代删除）----
function startFlag(it, flag) {
  if (!requireLogin()) return
  panel.value = { id: it.id, mode: 'flag' }
  flagForm.value = { flag, reason: '' }
}
async function submitFlag(it) {
  const reason = flagForm.value.reason.trim()
  if (!reason) return (error.value = '请填写标记原因')
  if (!window.confirm(`确认将「${it.company}」标记为「${flagForm.value.flag === 'invalid' ? '失效' : '重复'}」？`)) return
  saving.value = true
  error.value = ''
  try {
    await api.flagJob(it.id, { flag: flagForm.value.flag, reason })
    panel.value = { id: 0, mode: '' }
    await load()
  } catch (e) {
    error.value = e.message
  } finally {
    saving.value = false
  }
}
async function restore(it) {
  if (!requireLogin()) return
  try {
    await api.restoreJob(it.id)
    await load()
  } catch (e) {
    error.value = e.message
  }
}

// ---- 置顶（管理员）----
async function togglePin(it) {
  try {
    await api.pinJob(it.id, { pinned: !it.is_pinned })
    await load()
  } catch (e) {
    error.value = e.message
  }
}

// ---- 删除（管理员）----
async function remove(it) {
  if (!window.confirm(`确认永久删除「${it.company}」及全部版本历史？此操作不可恢复`)) return
  try {
    await api.deleteJob(it.id)
    await load()
  } catch (e) {
    error.value = e.message
  }
}

// ---- 版本历史 ----
async function toggleVersions(it) {
  if (panel.value.id === it.id && panel.value.mode === 'versions') {
    panel.value = { id: 0, mode: '' }
    return
  }
  panel.value = { id: it.id, mode: 'versions' }
  versionsLoading.value = true
  try {
    const { data } = await api.jobVersions(it.id)
    versions.value = data.items
  } catch (e) {
    error.value = e.message
  } finally {
    versionsLoading.value = false
  }
}
async function revertVersion(it, v) {
  if (!window.confirm(`确认恢复到 ${v.editor} 在 ${formatDate(v.created_at)} 提交的版本？`)) return
  try {
    await api.revertJob(it.id, { version_id: v.id })
    panel.value = { id: 0, mode: '' }
    await load()
  } catch (e) {
    error.value = e.message
  }
}

// ---- 公司评价 ----
async function toggleReviews(it) {
  if (panel.value.id === it.id && panel.value.mode === 'reviews') {
    panel.value = { id: 0, mode: '' }
    return
  }
  panel.value = { id: it.id, mode: 'reviews' }
  if (!reviews.value[it.id]) await loadReviews(it.id)
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
        <p class="job-sub">社区协作数据库：人人可查看，登录后即可新增、编辑、评价；每次修改都会记录版本、修改人与修改原因。</p>
      </div>
      <div class="list-tools">
        <button class="primary-btn" @click="startCreate">+ 新增公司</button>
      </div>
    </div>

    <!-- 状态筛选 -->
    <div class="job-filters">
      <button
        v-for="(label, key) in statusLabel"
        :key="key"
        class="filter-btn"
        :class="{ active: statusFilter === key }"
        @click="statusFilter = key; applyFilter()"
      >
        {{ label }} <b>{{ statusCounts[key] ?? 0 }}</b>
      </button>
    </div>

    <!-- 搜索栏：城市下拉 + 方向搜索 -->
    <div class="job-searchbar">
      <input
        v-model="cityKeyword"
        class="search-input"
        type="search"
        placeholder="按地点筛选，可下拉选择城市…"
        list="city-list"
        @keyup.enter="applyFilter"
      />
      <datalist id="city-list">
        <option v-for="c in cityOptions" :key="c" :value="c" />
      </datalist>
      <input
        v-model="industryKeyword"
        class="search-input"
        type="search"
        placeholder="搜索公司方向 / 产业链 / 公司名…"
        @keyup.enter="applyFilter"
      />
      <button class="filter-btn" @click="applyFilter">筛选</button>
      <button v-if="cityKeyword || industryKeyword" class="ghost-btn" @click="cityKeyword = ''; industryKeyword = ''; applyFilter()">
        清除
      </button>
      <span class="job-count">共 {{ filteredItems.length }} 条</span>
    </div>

    <p v-if="error" class="notice" style="margin-top: 14px">{{ error }}</p>
    <p v-if="!auth.isLoggedIn" class="notice" style="margin-top: 14px">
      登录后即可参与共享编辑与评价：添加公司、更新招聘信息、对公司填写评价（可匿名）。
    </p>

    <!-- 新增表单 -->
    <div v-if="newForm" class="job-card job-edit-card">
      <h3 style="margin-bottom: 12px">新增公司</h3>
      <div class="job-form">
        <label>公司 *<input v-model="newForm.company" maxlength="100" placeholder="公司名称" /></label>
        <label>产业链 / 光学方向<input v-model="newForm.industry" maxlength="200" placeholder="如：车载光学、AR/VR" /></label>
        <label>地点<input v-model="newForm.city" maxlength="100" placeholder="如：深圳、上海等" /></label>
        <label>内推码<input v-model="newForm.referral_code" maxlength="50" placeholder="如：NTxxxxxx" /></label>
        <label>上次核验<input v-model="newForm.verified_at" type="date" /></label>
        <label>来源备注<input v-model="newForm.edit_reason" maxlength="200" placeholder="可选：信息的来源/备注" /></label>
        <label class="job-form-wide">投递链接<textarea v-model="newForm.apply_link" maxlength="1000" rows="2" placeholder="每行一个链接" /></label>
      </div>
      <div class="job-form-actions">
        <button class="filter-btn" :disabled="saving" @click="submitCreate">保存</button>
        <button class="ghost-btn" @click="cancelCreate">取消</button>
      </div>
    </div>

    <div v-if="loading" class="empty-note" style="margin-top: 20px">加载中…</div>
    <div v-else-if="!filteredItems.length" class="empty-note" style="margin-top: 20px">
      {{ items.length ? '当前筛选条件下没有记录，试试其他筛选' : '还没有记录，点击「+ 新增公司」添加招聘信息' }}
    </div>

    <div v-else class="job-cards">
      <div v-for="it in filteredItems" :key="it.id" class="job-card" :class="{ 'job-card-dim': it.status !== 'active', 'job-card-pinned': it.is_pinned }">
        <!-- 头部：公司 + 徽章 -->
        <div class="job-card-head">
          <div class="job-card-title">
            <span v-if="it.is_pinned" class="badge badge-pinned">置顶</span>
            <span :class="['badge', statusClass[it.status]]">{{ statusLabel[it.status] }}</span>
            <h3>{{ it.company }}</h3>
          </div>
          <div class="job-card-actions">
            <button v-if="auth.isAdmin" class="ghost-btn" @click="togglePin(it)">{{ it.is_pinned ? '取消置顶' : '置顶' }}</button>
            <button v-if="auth.isAdmin" class="ghost-danger" @click="remove(it)">删除</button>
          </div>
        </div>

        <!-- 信息行 -->
        <div class="job-card-rows">
          <div v-if="it.industry" class="job-row"><b>方向</b><span>{{ it.industry }}</span></div>
          <div v-if="it.city" class="job-row"><b>地点</b><span>{{ it.city }}</span></div>
          <div v-if="it.referral_code" class="job-row"><b>内推码</b><span>{{ it.referral_code }}</span></div>
          <div class="job-row"><b>投递链接</b>
            <span>
              <template v-if="linkList(it.apply_link).length">
                <a v-for="(l, i) in linkList(it.apply_link)" :key="i" :href="l" target="_blank" rel="noopener"
                  class="text-link job-link">链接{{ i + 1 }}↗</a>
              </template>
              <span v-else class="job-dim">—</span>
            </span>
          </div>
        </div>

        <!-- 底部：核验信息 + 操作 -->
        <div class="job-card-foot">
          <div class="job-verify">
            <span>最近验证：<b>{{ it.updater || it.author }}</b></span>
            <span v-if="it.verified_at" class="job-dim">核验 {{ it.verified_at }}</span>
            <span v-if="verifyHint(it)" :class="verifyHint(it).cls">{{ verifyHint(it).text }}</span>
          </div>
          <div class="job-card-actions">
            <button class="filter-btn job-review-btn" @click="toggleReviews(it)">评价 <b>{{ it.review_count || 0 }}</b></button>
            <button class="filter-btn" @click="toggleVersions(it)">版本 {{ it.edit_count || 0 }}</button>
            <button class="filter-btn" @click="startEdit(it)">编辑</button>
            <button v-if="it.status !== 'active'" class="filter-btn" @click="restore(it)">恢复</button>
            <template v-if="it.status === 'active'">
              <button class="ghost-btn" @click="startFlag(it, 'invalid')">标记失效</button>
              <button class="ghost-btn" @click="startFlag(it, 'duplicate')">标记重复</button>
            </template>
          </div>
        </div>
        <div class="job-card-meta">
          <span class="job-dim">创建 {{ timeAgo(it.created_at) }} · {{ it.author }}</span>
          <span v-if="it.edit_reason" class="job-dim">最近修改：{{ it.edit_reason }}</span>
          <span class="job-dim" style="margin-left: auto">更新于 {{ timeAgo(it.updated_at) }}</span>
        </div>

        <!-- 操作面板 -->
        <div v-if="panel.id === it.id" class="job-panel">
          <!-- 编辑 -->
          <template v-if="panel.mode === 'edit' && editForm">
            <h4 class="job-panel-title">编辑「{{ it.company }}」</h4>
            <div class="job-form">
              <label>公司 *<input v-model="editForm.company" maxlength="100" /></label>
              <label>产业链 / 光学方向<input v-model="editForm.industry" maxlength="200" /></label>
              <label>地点<input v-model="editForm.city" maxlength="100" /></label>
              <label>内推码<input v-model="editForm.referral_code" maxlength="50" /></label>
              <label>上次核验<input v-model="editForm.verified_at" type="date" /></label>
              <label class="job-form-wide job-reason">
                修改原因 *<input v-model="editForm.edit_reason" maxlength="200" placeholder="说明这次改了什么、为什么改" />
              </label>
              <label class="job-form-wide">投递链接<textarea v-model="editForm.apply_link" maxlength="1000" rows="2" placeholder="每行一个链接" /></label>
            </div>
            <div class="job-form-actions">
              <button class="filter-btn" :disabled="saving" @click="submitEdit">保存修改</button>
              <button class="ghost-btn" @click="cancelEdit">取消</button>
            </div>
          </template>

          <!-- 标记失效/重复 -->
          <template v-else-if="panel.mode === 'flag'">
            <h4 class="job-panel-title">标记「{{ it.company }}」为{{ flagForm.flag === 'invalid' ? '失效' : '重复' }}</h4>
            <label class="job-reason">标记原因 *<input v-model="flagForm.reason" maxlength="200" placeholder="如：校招已结束 / 与另一家公司重复" /></label>
            <div class="job-form-actions">
              <button class="filter-btn" :disabled="saving" @click="submitFlag(it)">提交标记</button>
              <button class="ghost-btn" @click="panel = { id: 0, mode: '' }">取消</button>
            </div>
          </template>

          <!-- 版本历史 -->
          <template v-else-if="panel.mode === 'versions'">
            <h4 class="job-panel-title">版本历史</h4>
            <p v-if="versionsLoading" class="job-dim">加载中…</p>
            <div v-else-if="!versions.length" class="job-dim">暂无版本记录</div>
            <div v-for="v in versions" :key="v.id" class="job-version-item">
              <div class="job-version-head">
                <b>{{ v.editor }}</b>
                <span class="job-dim">{{ formatDate(v.created_at) }}</span>
                <span v-if="v.reason" class="job-version-reason">{{ v.reason }}</span>
                <button v-if="auth.isAdmin && panel.id === it.id" class="ghost-btn" style="margin-left: auto"
                  @click="revertVersion(it, v)">恢复此版本</button>
              </div>
              <div v-if="v.industry || v.city || v.referral_code" class="job-version-body job-dim">
                {{ [v.industry, v.city, v.referral_code].filter(Boolean).join(' ｜ ') }}
              </div>
            </div>
          </template>

          <!-- 评价 -->
          <template v-else-if="panel.mode === 'reviews'">
            <h4 class="job-panel-title">公司评价</h4>
            <p v-if="reviews[it.id]?.loading" class="job-dim">评价加载中…</p>
            <p v-else-if="!reviews[it.id]?.items?.length" class="job-dim">还没有评价，来写第一条吧</p>
            <div v-for="r in reviews[it.id]?.items" :key="r.id" class="job-review-item">
              <div class="job-review-head">
                <b>{{ r.author }}</b>
                <span class="job-dim">{{ timeAgo(r.created_at) }}</span>
                <button v-if="auth.isLoggedIn && (r.user_id === auth.user?.id || auth.isAdmin)" class="ghost-btn"
                  style="margin-left: auto; color: var(--danger)" @click="removeReview(it, r)">删除</button>
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
          </template>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.job-sub {
  color: var(--muted);
  font-size: 13px;
  margin-top: 2px;
}
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
.job-searchbar {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  align-items: center;
  margin-top: 12px;
}
.job-searchbar .search-input {
  max-width: 260px;
}
.job-count {
  font-size: 13px;
  color: var(--muted);
  margin-left: 4px;
}
.job-cards {
  display: flex;
  flex-direction: column;
  gap: 14px;
  margin-top: 16px;
}
.job-card {
  background: #fff;
  border: 1px solid var(--line);
  border-radius: 10px;
  padding: 16px 18px;
  box-shadow: 0 1px 2px rgba(15, 60, 95, 0.04);
}
.job-card-pinned {
  border-color: var(--primary);
  box-shadow: 0 0 0 1px var(--primary) inset;
}
.job-card-dim {
  opacity: 0.62;
}
.job-card-head {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  flex-wrap: wrap;
}
.job-card-title {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  min-width: 0;
}
.job-card-title h3 {
  font-size: 17px;
  margin: 0;
}
.job-card-actions {
  display: flex;
  gap: 8px;
  margin-left: auto;
  align-items: center;
}
.badge-pinned {
  background: #ffe9d6;
  color: #d2691e;
}
.job-card-rows {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 6px 24px;
  margin-top: 12px;
}
.job-row {
  display: flex;
  gap: 10px;
  font-size: 14px;
  line-height: 1.5;
}
.job-row b {
  color: var(--muted);
  font-weight: 500;
  flex-shrink: 0;
  min-width: 5em;
}
.job-row .job-pre {
  white-space: pre-wrap;
}
.job-card-foot {
  display: flex;
  gap: 12px;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  margin-top: 12px;
  padding-top: 12px;
  border-top: 1px dashed var(--line);
}
.job-verify {
  display: flex;
  gap: 10px;
  align-items: center;
  flex-wrap: wrap;
  font-size: 13px;
}
.job-expired {
  color: #c0392b;
  font-weight: 600;
}
.job-fresh {
  color: #2e8b57;
}
.job-card-meta {
  display: flex;
  gap: 14px;
  flex-wrap: wrap;
  font-size: 12px;
  margin-top: 8px;
}
.job-link {
  display: inline-block;
  margin-right: 8px;
  font-size: 13px;
}
.job-panel {
  margin-top: 12px;
  padding: 14px;
  background: var(--blue-soft);
  border: 1px solid var(--blue);
  border-radius: 8px;
}
.job-panel-title {
  margin: 0 0 12px;
  font-size: 14px;
  color: var(--primary);
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
  background: #fff;
}
.job-form .job-form-wide {
  grid-column: span 2;
}
.job-form .job-reason input {
  border-color: var(--primary);
}
.job-form-actions {
  display: flex;
  align-items: flex-end;
  gap: 8px;
  margin-top: 12px;
}
.job-reason {
  display: flex;
  flex-direction: column;
  gap: 4px;
  font-size: 12px;
  color: var(--muted);
}
.job-reason input {
  border: 1px solid var(--primary);
  border-radius: 5px;
  padding: 6px 8px;
  font-size: 13px;
  max-width: 420px;
}
.job-version-item {
  border-bottom: 1px dashed var(--line);
  padding: 8px 0;
}
.job-version-item:last-of-type {
  border-bottom: 0;
}
.job-version-head {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  font-size: 13px;
}
.job-version-reason {
  color: var(--ink);
  background: #fff;
  border-radius: 4px;
  padding: 1px 8px;
}
.job-version-body {
  margin-top: 4px;
  font-size: 12px;
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
.job-edit-card {
  margin-top: 16px;
}
@media (max-width: 760px) {
  .job-card-rows {
    grid-template-columns: 1fr;
  }
}
</style>
