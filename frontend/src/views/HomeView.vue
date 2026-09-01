<script setup>
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import api from '../api'
import { timeAgo } from '../utils/time'
import heroImg from '../assets/guangyanji-hero-image2.webp'

const router = useRouter()
const latestQuestions = ref([])
const latestArticles = ref([])
const latestPosts = ref([])
const latestJobs = ref([])

onMounted(async () => {
  try {
    const [q, a, p, j] = await Promise.all([
      api.questions({ page: 1, page_size: 5 }),
      api.articles({ page: 1, page_size: 4 }),
      api.forumPosts({ page: 1, page_size: 4 }),
      api.jobs({ status: 'active', page_size: 4 }),
    ])
    latestQuestions.value = q.data.items
    latestArticles.value = a.data.items
    latestPosts.value = p.data.items
    latestJobs.value = j.data.items.slice(0, 4)
  } catch {
    /* 首页加载失败不阻塞展示 */
  }
})
</script>

<template>
  <div>
    <section class="hero">
      <div class="hero-content">
        <p class="eyebrow">OPTICS · LEARNING · PRACTICE</p>
        <h1>光研集</h1>
        <p class="hero-lead">把光学问题，讲清楚、做出来。</p>
        <p class="hero-copy">
          面向光学学习者与工程师的问答社区。<br />从基础概念到工程实践，找到可靠答案，也留下你的经验。
        </p>
        <div class="hero-actions">
          <button class="primary-btn" @click="router.push('/ask/new')">
            开始提问 <span>→</span>
          </button>
          <router-link class="text-link" to="/knowledge">浏览知识库 <span>↗</span></router-link>
        </div>
      </div>
      <div class="hero-visual">
        <img :src="heroImg" alt="光研集" />
      </div>
    </section>

    <section class="section-tabs">
      <router-link class="portal-card" to="/ask">
        <h3>问题投稿</h3>
        <p>提出你在光学学习与工程实践中的问题，社区一起解答。支持公式与图片，采纳回答后问题标记为已解决。</p>
        <span class="portal-arrow">进入问答 →</span>
      </router-link>
      <router-link class="portal-card" to="/knowledge">
        <h3>知识库</h3>
        <p>沉淀可靠的光学知识文章：从基础概念到工程经验，按需检索，随手分享你的知识。</p>
        <span class="portal-arrow">浏览知识库 →</span>
      </router-link>
      <router-link class="portal-card" to="/forum">
        <h3>论坛</h3>
        <p>自由讨论光学相关话题：行业动态、学术前沿、学习资源与经验交流。</p>
        <span class="portal-arrow">进入论坛 →</span>
      </router-link>
      <router-link class="portal-card" to="/jobs">
        <h3>就业信息</h3>
        <p>2027 届光学公司校招共享数据库：人人可查看、可编辑，置顶新开公司，含真实评价与版本记录。</p>
        <span class="portal-arrow">进入表格 →</span>
      </router-link>
    </section>

    <section class="page" style="padding-top: 10px">
      <div class="section-heading">
        <div>
          <p class="eyebrow">COMMUNITY</p>
          <h2>最新动态</h2>
        </div>
        <router-link class="text-link" to="/ask">全部投稿 ↗</router-link>
      </div>
      <div class="feed">
        <div v-for="q in latestQuestions" :key="q.id" class="card clickable-card" @click="router.push(`/ask/${q.id}`)">
          <h3>{{ q.title }}</h3>
          <div class="meta">
            <span class="badge">{{ q.category_name }}</span>
            <span>回答 {{ q.answer_count }}</span>
            <span style="margin-left: auto">{{ q.author }} · {{ timeAgo(q.created_at) }}</span>
          </div>
        </div>
      </div>

      <div class="section-heading" style="margin-top: 36px">
        <div>
          <p class="eyebrow">KNOWLEDGE BASE</p>
          <h2>知识库精选</h2>
        </div>
        <router-link class="text-link" to="/knowledge">全部文章 ↗</router-link>
      </div>
      <div class="feed">
        <div v-for="a in latestArticles" :key="a.id" class="card clickable-card"
          @click="router.push(`/knowledge/${a.id}`)">
          <h3>{{ a.title }}</h3>
          <p class="excerpt">{{ a.summary || a.body.slice(0, 120) }}</p>
          <div class="meta">
            <span class="badge badge-teal">知识文章</span>
            <span>浏览 {{ a.views }}</span>
            <span style="margin-left: auto">{{ a.author }} · {{ timeAgo(a.created_at) }}</span>
          </div>
        </div>
      </div>

      <div class="section-heading" style="margin-top: 36px">
        <div>
          <p class="eyebrow">FORUM</p>
          <h2>论坛热帖</h2>
        </div>
        <router-link class="text-link" to="/forum">全部帖子 ↗</router-link>
      </div>
      <div class="feed">
        <div v-for="p in latestPosts" :key="p.id" class="card clickable-card" @click="router.push(`/forum/${p.id}`)">
          <h3>{{ p.title }}</h3>
          <div class="meta">
            <span class="badge badge-teal">{{ p.board_name }}</span>
            <span>回复 {{ p.reply_count }}</span>
            <span style="margin-left: auto">{{ p.author }} · {{ timeAgo(p.created_at) }}</span>
          </div>
        </div>
      </div>

      <div class="section-heading" style="margin-top: 36px">
        <div>
          <p class="eyebrow">JOBS</p>
          <h2>最新招聘</h2>
        </div>
        <router-link class="text-link" to="/jobs">进入就业表格 ↗</router-link>
      </div>
      <div class="feed">
        <div v-for="j in latestJobs" :key="j.id" class="card clickable-card" @click="router.push('/jobs')">
          <h3>
            <span v-if="j.is_pinned" class="badge badge-pinned">置顶</span>
            {{ j.company }}
          </h3>
          <p class="excerpt">
            {{ [j.industry, j.city, j.current_status].filter(Boolean).join(' ｜ ') || j.positions_27 || '点击查看招聘详情' }}
          </p>
          <div class="meta">
            <span class="badge badge-teal">就业信息</span>
            <span>核验 {{ j.verified_at || '—' }}</span>
            <span style="margin-left: auto">更新于 {{ timeAgo(j.updated_at) }}</span>
          </div>
        </div>
        <div v-if="!latestJobs.length" class="card">
          <p class="excerpt" style="margin: 0">就业信息表格还在填充中，<router-link class="text-link" to="/jobs">点击进入表格查看或添加</router-link></p>
        </div>
      </div>
    </section>
  </div>
</template>

<style scoped>
.badge-pinned {
  background: #ffe9d6;
  color: #d2691e;
  margin-right: 6px;
}
</style>
