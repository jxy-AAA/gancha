import { createRouter, createWebHistory } from 'vue-router'
import { setSeo } from '../utils/seo'

const routes = [
  { path: '/', name: 'home', component: () => import('../views/HomeView.vue') },
  { path: '/ask', name: 'ask', component: () => import('../views/AskView.vue') },
  { path: '/ask/new', name: 'ask-new', component: () => import('../views/AskNewView.vue') },
  { path: '/ask/:id', name: 'ask-detail', component: () => import('../views/AskDetailView.vue') },
  { path: '/knowledge', name: 'knowledge', component: () => import('../views/KnowledgeView.vue') },
  { path: '/knowledge/new', name: 'knowledge-new', component: () => import('../views/KnowledgeNewView.vue') },
  { path: '/knowledge/:id/edit', name: 'knowledge-edit', component: () => import('../views/KnowledgeNewView.vue') },
  { path: '/knowledge/:id', name: 'knowledge-detail', component: () => import('../views/KnowledgeDetailView.vue') },
  { path: '/forum', name: 'forum', component: () => import('../views/ForumView.vue') },
  { path: '/forum/new', name: 'forum-new', component: () => import('../views/ForumNewView.vue') },
  { path: '/forum/:id', name: 'forum-detail', component: () => import('../views/ForumDetailView.vue') },
  { path: '/jobs', name: 'jobs', component: () => import('../views/JobsView.vue') },
  { path: '/login', name: 'login', component: () => import('../views/LoginView.vue') },
  { path: '/register', name: 'register', component: () => import('../views/RegisterView.vue') },
  { path: '/profile', name: 'profile', component: () => import('../views/ProfileView.vue') },
  { path: '/admin', name: 'admin', component: () => import('../views/admin/AdminView.vue'), meta: { admin: true } },
  { path: '/:pathMatch(.*)*', redirect: '/' },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
  scrollBehavior(to, from, savedPosition) {
    if (to.hash) return { el: to.hash, behavior: 'smooth', top: 80 }
    return savedPosition || { top: 0 }
  },
})

// 路由级 title/description（详情页会用真实标题再覆盖一次）
const pageMeta = {
  home: {},
  ask: { title: '问题投稿', description: '光学设计、光学考研、Zemax 使用等方向的提问与解答。' },
  'ask-new': { title: '发起提问', description: '在棱语 OptiTalk 提出你的光学问题，社区一起解答。' },
  'ask-detail': { title: '问题详情' },
  knowledge: { title: '知识库', description: '光学基础概念与工程实践文章，按需检索、随手分享。' },
  'knowledge-new': { title: '撰写文章', description: '在棱语 OptiTalk 知识库分享你的光学知识与经验。' },
  'knowledge-edit': { title: '编辑文章' },
  'knowledge-detail': { title: '文章详情' },
  forum: { title: '论坛交流', description: '光学行业动态、学术讨论、学习资源与经验交流。' },
  'forum-new': { title: '发布帖子', description: '在棱语 OptiTalk 论坛发起光学话题讨论。' },
  'forum-detail': { title: '帖子详情' },
  jobs: {
    title: '就业信息｜2027 届光学公司校招共享数据库',
    description: '2027 届光学公司校招信息共享数据库：含公司方向、城市、校招状态、内推码与投递链接，人人可查看、可编辑。',
  },
  login: { title: '登录' },
  register: { title: '注册' },
  profile: { title: '个人中心' },
  admin: { title: '管理后台' },
}

router.afterEach((to) => {
  const meta = pageMeta[to.name] || {}
  setSeo({ title: meta.title, description: meta.description, path: to.fullPath, type: 'website' })
})

export default router
