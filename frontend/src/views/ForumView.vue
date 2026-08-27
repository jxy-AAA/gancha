<script setup>
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import api from '../api'
import Pagination from '../components/Pagination.vue'
import { timeAgo } from '../utils/time'

const router = useRouter()
const boards = ref([])
const items = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = 10
const boardId = ref(0)
const search = ref('')
const loading = ref(false)

async function load() {
  loading.value = true
  try {
    const { data } = await api.forumPosts({
      page: page.value,
      page_size: pageSize,
      board_id: boardId.value || undefined,
      search: search.value || undefined,
    })
    items.value = data.items
    total.value = data.total
  } finally {
    loading.value = false
  }
}
function onBoard(id) {
  boardId.value = id
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
onMounted(async () => {
  load()
  try {
    const { data } = await api.boards()
    boards.value = data.items
  } catch { /* 忽略 */ }
})
</script>

<template>
  <div class="page">
    <div class="section-heading">
      <div>
        <p class="eyebrow">FORUM</p>
        <h2>论坛</h2>
      </div>
      <div class="list-tools">
        <input v-model="search" class="search-input" type="search" placeholder="搜索帖子"
          @keyup.enter="onSearch" />
        <button class="filter-btn" @click="onSearch">搜索</button>
        <router-link class="primary-btn" to="/forum/new">+ 发帖</router-link>
      </div>
    </div>

    <div class="feed" style="margin-bottom: 22px">
      <div class="card clickable-card" @click="router.push('/jobs')">
        <h3 style="font-size: 15px">
          就业信息
          <span style="color: var(--muted); font-weight: 400; font-size: 13px; margin-left: 10px">公司 / 岗位 / 投递进度共享表格，人人可编辑</span>
          <span style="margin-left: auto; color: var(--muted); font-weight: 600; font-size: 12px">进入表格 →</span>
        </h3>
      </div>
      <div v-for="b in boards" :key="b.id" class="card" style="cursor: pointer" @click="onBoard(b.id === boardId ? 0 : b.id)">
        <h3 style="font-size: 15px">
          {{ b.name }}
          <span style="color: var(--muted); font-weight: 400; font-size: 13px; margin-left: 10px">{{ b.description }}</span>
          <span style="margin-left: auto; color: var(--muted); font-weight: 400; font-size: 12px">{{ b.post_count }} 帖</span>
        </h3>
      </div>
    </div>

    <div v-if="loading" class="empty-note">加载中…</div>
    <div v-else-if="!items.length" class="empty-note">这个板块还没有帖子，来发第一帖吧</div>
    <div v-else class="feed">
      <div v-for="p in items" :key="p.id" class="card clickable-card" @click="router.push(`/forum/${p.id}`)">
        <h3>{{ p.title }}</h3>
        <div class="meta">
          <span class="badge badge-teal">{{ p.board_name }}</span>
          <span>回复 {{ p.reply_count }}</span>
          <span>浏览 {{ p.views }}</span>
          <span style="margin-left: auto">{{ p.author }} · {{ timeAgo(p.created_at) }}</span>
        </div>
      </div>
    </div>
    <Pagination :page="page" :total="total" :page-size="pageSize" @change="onPage" />
  </div>
</template>
