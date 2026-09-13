<script setup>
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import api from '../api'
import { useAuthStore } from '../stores/auth'
import { timeAgo, formatDate } from '../utils/time'
import Pagination from '../components/Pagination.vue'

const router = useRouter()
const auth = useAuthStore()

// 分页（服务端）：每页条数 + 分片渲染批大小（控制 DOM 规模，避免整页卡顿）
const PAGE_SIZE = 50
const CHUNK = 15

const items = ref([])
const loading = ref(false)
const error = ref('')
const saving = ref(false)

const page = ref(1)
const total = ref(0)
const stats = ref(null)
const pinnedIds = ref([])
const visibleCount = ref(CHUNK)
const sentinel = ref(null)
let observer = null

// 筛选（全部由服务端执行）
const statusFilter = ref('active') // active | invalid | duplicate | all
const cityKeyword = ref('')
const industryKeyword = ref('')
const campusFilter = ref('all') // all | 已开启
const appliedFilter = ref('all') // all | applied | not

// 每张卡片的操作面板：panel.id + panel.mode(edit/flag/versions/reviews)
const panel = ref({ id: 0, mode: '' })
const editForm = ref(null)
const flagForm = ref({ flag: 'invalid', reason: '' })
const versions = ref([])
const versionsLoading = ref(false)
const reviews = ref({}) // id -> { items, loading }
const reviewText = ref({})
const reviewAnon = ref({})
const appliedMap = ref({}) // id -> 我是否已投递（勾选框数据源）
const appliedPending = ref({}) // id -> 请求中（防连点）

const statusLabel = { active: '正常', invalid: '已失效', duplicate: '重复', all: '全部' }
const statusClass = { active: 'badge-green', invalid: 'badge-gray', duplicate: 'badge-teal' }

// 城市下拉：服务端 DISTINCT（分页后不能再从当前页数据里推）
const cityOptions = ref([])
async function loadCities() {
  try {
    const { data } = await api.jobCities()
    cityOptions.value = data.items || []
  } catch {
    cityOptions.value = []
  }
}

// 分面计数来自服务端（各维度剔除自身条件），与分页无关
const statusCounts = computed(() => stats.value?.status || { active: 0, invalid: 0, duplicate: 0, all: 0 })
const campusCounts = computed(() => stats.value?.campus || { all: 0, open: 0 })
const myCounts = computed(() => stats.value?.applied || { all: 0, applied: 0, not: 0 })

// 分片渲染：只挂载前 visibleCount 张卡片，滚动到底部再增量挂载
const visibleItems = computed(() => items.value.slice(0, visibleCount.value))

const linkList = (links) => String(links || '').split(/\n+/).map((s) => s.trim()).filter(Boolean)

async function load() {
  loading.value = true
  try {
    const { data } = await api.jobs({
      page: page.value,
      page_size: PAGE_SIZE,
      status: statusFilter.value,
      campus_status: campusFilter.value === 'all' ? undefined : campusFilter.value,
      applied: appliedFilter.value === 'all' ? undefined : appliedFilter.value,
      city: cityKeyword.value || undefined,
      industry: industryKeyword.value || undefined,
    })
    items.value = data.items || []
    total.value = data.total || 0
    stats.value = data.stats || null
    pinnedIds.value = data.pinned_ids || []
    visibleCount.value = CHUNK
    const m = {}
    for (const it of items.value) if (it.my_applied) m[it.id] = true
    appliedMap.value = m
    error.value = ''
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}

// 换筛选/翻页：重置面板与分片计数，回到列表顶部
function reloadFromStart() {
  panel.value = { id: 0, mode: '' }
  visibleCount.value = CHUNK
  window.scrollTo({ top: 0 })
  load()
}

function applyFilter() {
  page.value = 1
  reloadFromStart()
}

function changePage(p) {
  page.value = p
  reloadFromStart()
}

function requireLogin() {
  if (!auth.isLoggedIn) {
    router.push('/login?redirect=/jobs')
    return false
  }
  return true
}

// 我的投递勾选：仅记录个人状态，不触发整表刷新（不影响排序）
async function toggleApplied(it) {
  const target = !!appliedMap.value[it.id]
  if (!auth.isLoggedIn) {
    appliedMap.value = { ...appliedMap.value, [it.id]: false }
    requireLogin()
    return
  }
  if (appliedPending.value[it.id]) return
  appliedPending.value = { ...appliedPending.value, [it.id]: true }
  try {
    await api.setJobApplied(it.id, { applied: target })
    // 计数本地微调，避免每勾一次都整页刷新；处于「我的状态」筛选时需重查（该行可能离开当前条件）
    if (stats.value?.applied) {
      const s = stats.value.applied
      stats.value = { ...stats.value, applied: { all: s.all, applied: s.applied + (target ? 1 : -1), not: s.not + (target ? -1 : 1) } }
    }
    if (appliedFilter.value !== 'all') await load()
  } catch (e) {
    error.value = e.message
    appliedMap.value = { ...appliedMap.value, [it.id]: !target }
  } finally {
    appliedPending.value = { ...appliedPending.value, [it.id]: false }
  }
}

// ---- 新增 ----
const newForm = ref(null)
function startCreate() {
  if (!requireLogin()) return
  newForm.value = {
    company: '', industry: '', city: '', apply_link: '',
    referral_code: '', verified_at: '', campus_status: '待核验', edit_reason: '',
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
    campus_status: it.campus_status || '待核验', edit_reason: '',
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

async function movePin(it, dir) {
  try {
    await api.moveJob(it.id, { direction: dir })
    await load()
  } catch (e) {
    error.value = e.message
  }
}

// 后端返回的置顶行完整顺序（跨页）：首条不可上移、末条不可下移
const isPinnedEdge = (it, dir) => {
  if (!pinnedIds.value.length) return true
  return dir === 'up' ? it.id === pinnedIds.value[0] : it.id === pinnedIds.value[pinnedIds.value.length - 1]
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

// 哨兵进入视口就再挂载一批卡片，直到本页全部渲染
watch(sentinel, (el) => {
  observer?.disconnect()
  observer = null
  if (!el) return
  observer = new IntersectionObserver(
    (entries) => {
      if (entries.some((e) => e.isIntersecting) && visibleCount.value < items.value.length) {
        visibleCount.value += CHUNK
      }
    },
    { rootMargin: '400px' },
  )
  observer.observe(el)
})

onUnmounted(() => observer?.disconnect())

onMounted(() => {
  loadCities()
  load()
})
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

    <!-- 校招状态筛选 -->
    <div class="job-filters job-cs-filter">
      <span class="job-cs-label">校招状态</span>
      <button class="filter-btn" :class="{ active: campusFilter === 'all' }" @click="campusFilter = 'all'; applyFilter()">
        全部 <b>{{ campusCounts.all }}</b>
      </button>
      <button class="filter-btn" :class="{ active: campusFilter === '已开启' }" @click="campusFilter = '已开启'; applyFilter()">
        只看已开启 <b>{{ campusCounts.open }}</b>
      </button>
    </div>

    <!-- 我的投递状态筛选（登录后可见） -->
    <div v-if="auth.isLoggedIn" class="job-filters job-cs-filter">
      <span class="job-cs-label">我的状态</span>
      <button class="filter-btn" :class="{ active: appliedFilter === 'all' }" @click="appliedFilter = 'all'; applyFilter()">
        全部 <b>{{ myCounts.all }}</b>
      </button>
      <button class="filter-btn" :class="{ active: appliedFilter === 'applied' }" @click="appliedFilter = 'applied'; applyFilter()">
        已投递 <b>{{ myCounts.applied }}</b>
      </button>
      <button class="filter-btn" :class="{ active: appliedFilter === 'not' }" @click="appliedFilter = 'not'; applyFilter()">
        未投递 <b>{{ myCounts.not }}</b>
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
      <span class="job-count">共 {{ total }} 条</span>
    </div>

    <p v-if="error" class="notice" style="margin-top: 14px">{{ error }}</p>
    <p v-if="!auth.isLoggedIn" class="notice" style="margin-top: 14px">
      登录后即可参与共享编辑与评价：添加公司、更新招聘信息、对公司填写评价（可匿名），
      还能在每家公司勾选「我的投递」记录个人进度（仅自己可见）。
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
        <label v-if="auth.isAdmin">校招状态<select v-model="newForm.campus_status"><option value="待核验">待核验</option><option value="已开启">已开启</option></select></label>
        <label>来源备注<input v-model="newForm.edit_reason" maxlength="200" placeholder="可选：信息的来源/备注" /></label>
        <label class="job-form-wide">投递链接<textarea v-model="newForm.apply_link" maxlength="1000" rows="2" placeholder="每行一个链接" /></label>
      </div>
      <div class="job-form-actions">
        <button class="filter-btn" :disabled="saving" @click="submitCreate">保存</button>
        <button class="ghost-btn" @click="cancelCreate">取消</button>
      </div>
    </div>

    <div v-if="loading" class="empty-note" style="margin-top: 20px">加载中…</div>
    <div v-else-if="!items.length" class="empty-note" style="margin-top: 20px">
      {{ total ? '当前筛选条件下没有记录，试试其他筛选' : '还没有记录，点击「+ 新增公司」添加招聘信息' }}
    </div>

    <div v-else class="job-cards">
      <div v-for="it in visibleItems" :key="it.id" class="job-card" :class="{ 'job-card-dim': it.status !== 'active', 'job-card-pinned': it.is_pinned }">
        <!-- 头部：公司 + 徽章 -->
        <div class="job-card-head">
          <div class="job-card-title">
            <span v-if="it.is_pinned" class="badge badge-pinned">置顶</span>
            <span :class="['badge', statusClass[it.status]]">{{ statusLabel[it.status] }}</span>
            <h3>{{ it.company }}</h3>
          </div>
          <div class="job-card-actions">
            <template v-if="auth.isAdmin && it.is_pinned">
              <button class="ghost-btn" :disabled="isPinnedEdge(it, 'up')" title="置顶区上移" @click="movePin(it, 'up')">↑ 上移</button>
              <button class="ghost-btn" :disabled="isPinnedEdge(it, 'down')" title="置顶区下移" @click="movePin(it, 'down')">↓ 下移</button>
            </template>
            <button v-if="auth.isAdmin" class="ghost-btn" @click="togglePin(it)">{{ it.is_pinned ? '取消置顶' : '置顶' }}</button>
            <button v-if="auth.isAdmin" class="ghost-danger" @click="remove(it)">删除</button>
          </div>
        </div>

        <!-- 信息行 -->
        <div class="job-card-rows">
          <div v-if="it.industry" class="job-row"><b>方向</b><span>{{ it.industry }}</span></div>
          <div v-if="it.city" class="job-row"><b>地点</b><span>{{ it.city }}</span></div>
          <div v-if="it.referral_code" class="job-row"><b>内推码</b><span>{{ it.referral_code }}</span></div>
          <div class="job-row"><b>校招状态</b><span :class="it.campus_status === '已开启' ? 'text-cs-open' : 'text-cs-pending'">{{ it.campus_status === '已开启' ? '已开启' : '待核验' }}</span></div>
          <div class="job-row"><b>我的投递</b>
            <span>
              <label class="job-apply-check" :class="{ checked: appliedMap[it.id] }">
                <input
                  v-model="appliedMap[it.id]"
                  type="checkbox"
                  :disabled="!!appliedPending[it.id]"
                  @change="toggleApplied(it)"
                />
                已投递
              </label>
              <span v-if="!auth.isLoggedIn" class="job-dim">（仅自己可见，登录后可标记）</span>
            </span>
          </div>
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

        <!-- 底部：操作 -->
        <div class="job-card-foot">
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
              <label v-if="auth.isAdmin">校招状态<select v-model="editForm.campus_status"><option value="待核验">待核验</option><option value="已开启">已开启</option></select></label>
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
      <div v-if="visibleCount < items.length" ref="sentinel" class="job-more">加载更多…</div>
    </div>
    <Pagination :page="page" :total="total" :page-size="PAGE_SIZE" @change="changePage" />
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
.job-cs-filter {
  margin-top: 6px;
}
.job-cs-label {
  align-self: center;
  font-size: 12px;
  color: var(--muted);
  margin-right: 2px;
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
.text-cs-open {
  color: #1e7e43;
  font-weight: 500;
}
.text-cs-pending {
  color: #6b7478;
}
.ghost-btn:disabled {
  opacity: 0.35;
  cursor: not-allowed;
}
.ghost-btn:disabled:hover {
  color: var(--ink);
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
  justify-content: flex-end;
  flex-wrap: wrap;
  margin-top: 12px;
  padding-top: 12px;
  border-top: 1px dashed var(--line);
}
.job-more {
  text-align: center;
  color: var(--muted);
  font-size: 13px;
  padding: 10px 0;
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
.job-apply-check {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  cursor: pointer;
  font-size: 13px;
  color: var(--muted);
}
.job-apply-check.checked {
  color: var(--primary);
  font-weight: 500;
}
.job-apply-check input {
  width: 15px;
  height: 15px;
  accent-color: var(--primary);
  cursor: pointer;
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
