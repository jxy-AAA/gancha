import axios from 'axios'

const api = axios.create({ baseURL: '/api', withCredentials: true })

api.interceptors.response.use(
  (res) => res,
  (err) => {
    const msg = err.response?.data?.error || '网络错误，请稍后再试'
    return Promise.reject(new Error(msg))
  },
)

export default {
  // 认证
  register: (d) => api.post('/auth/register', d),
  login: (d) => api.post('/auth/login', d),
  logout: () => api.post('/auth/logout'),
  me: () => api.get('/auth/me'),
  updateMe: (d) => api.put('/auth/me', d),
  changePassword: (d) => api.post('/auth/password', d),

  // 问答
  questions: (params) => api.get('/questions', { params }),
  question: (id) => api.get(`/questions/${id}`),
  createQuestion: (d) => api.post('/questions', d),
  updateQuestion: (id, d) => api.put(`/questions/${id}`, d),
  deleteQuestion: (id) => api.delete(`/questions/${id}`),
  viewQuestion: (id) => api.post(`/questions/${id}/view`),
  closeQuestion: (id) => api.post(`/questions/${id}/close`),
  toggleBookmark: (id) => api.post(`/questions/${id}/bookmark`),
  bookmarks: () => api.get('/bookmarks'),
  toggleFollow: (id) => api.post(`/questions/${id}/follow`),

  // 回答与评论
  answers: (qid) => api.get(`/questions/${qid}/answers`),
  createAnswer: (qid, d) => api.post(`/questions/${qid}/answers`, d),
  updateAnswer: (id, d) => api.put(`/answers/${id}`, d),
  deleteAnswer: (id) => api.delete(`/answers/${id}`),
  acceptAnswer: (id) => api.post(`/answers/${id}/accept`),
  comments: (qid) => api.get(`/questions/${qid}/comments`),
  createComment: (qid, d) => api.post(`/questions/${qid}/comments`, d),
  deleteComment: (id) => api.delete(`/comments/${id}`),

  // 投票
  toggleVote: (d) => api.post('/votes', d),
  voteStatus: (d) => api.post('/votes/status', d),

  // 文章评论
  articleComments: (id) => api.get(`/articles/${id}/comments`),
  createArticleComment: (id, d) => api.post(`/articles/${id}/comments`, d),
  deleteArticleComment: (id) => api.delete(`/article-comments/${id}`),

  // 知识库
  articles: (params) => api.get('/articles', { params }),
  article: (id) => api.get(`/articles/${id}`),
  createArticle: (d) => api.post('/articles', d),
  updateArticle: (id, d) => api.put(`/articles/${id}`, d),
  deleteArticle: (id) => api.delete(`/articles/${id}`),
  viewArticle: (id) => api.post(`/articles/${id}/view`),

  // 论坛
  boards: () => api.get('/boards'),
  forumPosts: (params) => api.get('/forum/posts', { params }),
  forumPost: (id) => api.get(`/forum/posts/${id}`),
  createForumPost: (d) => api.post('/forum/posts', d),
  updateForumPost: (id, d) => api.put(`/forum/posts/${id}`, d),
  deleteForumPost: (id) => api.delete(`/forum/posts/${id}`),
  viewForumPost: (id) => api.post(`/forum/posts/${id}/view`),
  pinForumPost: (id, d) => api.post(`/forum/posts/${id}/pin`, d),
  createForumReply: (id, d) => api.post(`/forum/posts/${id}/replies`, d),
  updateForumReply: (id, d) => api.put(`/forum/replies/${id}`, d),
  deleteForumReply: (id) => api.delete(`/forum/replies/${id}`),

  // 就业共享表格（2027 届公司招聘信息，社区协作数据库）
  jobs: (params) => api.get('/jobs', { params }),
  createJob: (d) => api.post('/jobs', d),
  updateJob: (id, d) => api.put(`/jobs/${id}`, d),
  flagJob: (id, d) => api.post(`/jobs/${id}/flag`, d),
  restoreJob: (id) => api.post(`/jobs/${id}/restore`),
  revertJob: (id, d) => api.post(`/jobs/${id}/revert`, d),
  pinJob: (id, d) => api.post(`/jobs/${id}/pin`, d),
  jobVersions: (id) => api.get(`/jobs/${id}/versions`),
  deleteJob: (id) => api.delete(`/jobs/${id}`),
  jobReviews: (id) => api.get(`/jobs/${id}/reviews`),
  createJobReview: (id, d) => api.post(`/jobs/${id}/reviews`, d),
  deleteJobReview: (id) => api.delete(`/jobs/reviews/${id}`),

  // 通知
  notifications: () => api.get('/notifications'),
  readNotifications: () => api.post('/notifications/read'),

  // 上传
  upload: (files) => {
    const fd = new FormData()
    for (const f of files) fd.append('files', f)
    return api.post('/uploads', fd, { headers: { 'Content-Type': 'multipart/form-data' } })
  },
  deleteUpload: (name) => api.delete(`/uploads/${encodeURIComponent(name)}`),

  // 分类与管理
  publicCategories: () => api.get('/categories'),
  categories: () => api.get('/admin/categories'),
  createCategory: (d) => api.post('/admin/categories', d),
  updateCategory: (id, d) => api.put(`/admin/categories/${id}`, d),
  deleteCategory: (id) => api.delete(`/admin/categories/${id}`),
  adminStats: () => api.get('/admin/stats'),
  adminUsers: (params) => api.get('/admin/users', { params }),
  updateAdminUser: (id, d) => api.put(`/admin/users/${id}`, d),
  deleteAdminUser: (id) => api.delete(`/admin/users/${id}`),
  adminAudit: (params) => api.get('/admin/audit', { params }),
}
