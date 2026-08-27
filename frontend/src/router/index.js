import { createRouter, createWebHistory } from 'vue-router'

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
  scrollBehavior: () => ({ top: 0 }),
})

export default router
