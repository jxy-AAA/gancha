<script setup>
import { onMounted, ref } from 'vue'
import api from '../api'
import Pagination from '../components/Pagination.vue'
import { timeAgo } from '../utils/time'

const items = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = 10
const search = ref('')
const loading = ref(false)

async function load() {
  loading.value = true
  try {
    const { data } = await api.articles({
      page: page.value,
      page_size: pageSize,
      search: search.value || undefined,
    })
    items.value = data.items
    total.value = data.total
  } finally {
    loading.value = false
  }
}
function onSearch() {
  page.value = 1
  load()
}
function onPage(p) {
  page.value = p
  load()
}
onMounted(load)
</script>

<template>
  <div class="page">
    <div class="section-heading">
      <div>
        <p class="eyebrow">KNOWLEDGE BASE</p>
        <h2>知识库</h2>
      </div>
      <div class="list-tools">
        <input v-model="search" class="search-input" type="search" placeholder="搜索文章标题或内容"
          @keyup.enter="onSearch" />
        <button class="filter-btn" @click="onSearch">搜索</button>
        <router-link class="primary-btn" to="/knowledge/new">+ 投稿文章</router-link>
      </div>
    </div>
    <div v-if="loading" class="empty-note">加载中…</div>
    <div v-else-if="!items.length" class="empty-note">知识库还是空的，来投递第一篇文章吧</div>
    <div v-else class="feed">
      <div v-for="a in items" :key="a.id" class="card">
        <h3><router-link :to="`/knowledge/${a.id}`">{{ a.title }}</router-link></h3>
        <p class="excerpt">{{ a.summary || a.body.replace(/[#*`$>\[\]|]/g, '').slice(0, 160) }}</p>
        <div class="meta">
          <span class="badge badge-teal">知识文章</span>
          <span v-if="a.tags">
            <span v-for="t in a.tags.split(',').filter(Boolean).slice(0, 4)" :key="t" class="tag">{{ t.trim() }}</span>
          </span>
          <span>▲ {{ a.score ?? 0 }}</span>
          <span>💬 {{ a.comment_count ?? 0 }}</span>
          <span style="margin-left: auto">
            <img v-if="a.author_avatar" :src="a.author_avatar" class="avatar" alt="" />
            {{ a.author }} · 浏览 {{ a.views }} · {{ timeAgo(a.created_at) }}
          </span>
        </div>
      </div>
    </div>
    <Pagination :page="page" :total="total" :page-size="pageSize" @change="onPage" />
  </div>
</template>
