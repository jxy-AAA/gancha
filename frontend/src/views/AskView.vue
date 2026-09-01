<script setup>
import { onMounted, ref, watch } from 'vue'
import api from '../api'
import QuestionCard from '../components/QuestionCard.vue'
import Pagination from '../components/Pagination.vue'

const items = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = 10
const categories = ref([])
const categoryId = ref(0)
const statusFilter = ref('')
const search = ref('')
const sort = ref('newest')
const loading = ref(false)

const statusOptions = [
  { value: '', label: '全部状态' },
  { value: 'open', label: '待解决' },
  { value: 'solved', label: '已解决' },
  { value: 'closed', label: '已关闭' },
]

async function load() {
  loading.value = true
  try {
    const { data } = await api.questions({
      page: page.value,
      page_size: pageSize,
      category: categoryId.value || undefined,
      status: statusFilter.value || undefined,
      search: search.value || undefined,
      sort: sort.value,
    })
    items.value = data.items
    total.value = data.total
  } finally {
    loading.value = false
  }
}

function onCategory(id) {
  categoryId.value = id
  page.value = 1
  load()
}
function onSearch() {
  page.value = 1
  load()
}
function onPage(p) {
  page.value = p
  load()
}
watch([sort, statusFilter], () => {
  page.value = 1
  load()
})

onMounted(async () => {
  load()
  try {
    const { data } = await api.publicCategories()
    categories.value = data.items
  } catch {
    /* 分类加载失败不影响列表 */
  }
})
</script>

<template>
  <div class="page">
    <div class="section-heading">
      <div>
        <p class="eyebrow">COMMUNITY</p>
        <h2>问题投稿</h2>
      </div>
      <div class="list-tools">
        <input v-model="search" class="search-input" type="search" placeholder="搜索标题、正文或标签"
          @keyup.enter="onSearch" />
        <button class="filter-btn" @click="onSearch">搜索</button>
        <select v-model="sort" class="filter-btn">
          <option value="newest">最新发布</option>
          <option value="updated">最后回复</option>
          <option value="popular">最热</option>
        </select>
        <select v-model="statusFilter" class="filter-btn">
          <option v-for="s in statusOptions" :key="s.value" :value="s.value">{{ s.label }}</option>
        </select>
        <router-link class="primary-btn" to="/ask/new">+ 发布投稿</router-link>
      </div>
    </div>
    <div class="layout">
      <div>
        <div v-if="loading" class="empty-note">加载中…</div>
        <div v-else-if="!items.length" class="empty-note">还没有投稿，来发布第一个问题吧</div>
        <div v-else class="feed">
          <QuestionCard v-for="q in items" :key="q.id" :q="q" />
        </div>
        <Pagination :page="page" :total="total" :page-size="pageSize" @change="onPage" />
      </div>
      <aside>
        <div class="side-panel">
          <div class="side-title">投稿分类</div>
          <a :class="{ active: categoryId === 0 }" @click="onCategory(0)">全部分类</a>
          <a v-for="c in categories" :key="c.id" :class="{ active: categoryId === c.id }" @click="onCategory(c.id)">
            {{ c.name }}
          </a>
        </div>
        <div class="side-panel">
          <div class="side-title">发布原则</div>
          <p>请提供必要参数、问题背景和已经尝试的方法。引用资料时请注明来源。</p>
        </div>
      </aside>
    </div>
  </div>
</template>
