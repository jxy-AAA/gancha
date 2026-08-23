import { defineStore } from 'pinia'
import api from '../api'

export const useAuthStore = defineStore('auth', {
  state: () => ({
    user: JSON.parse(localStorage.getItem('guangyanji_user') || 'null'),
  }),
  getters: {
    isLoggedIn: (s) => !!s.user,
    isAdmin: (s) => s.user?.role === 'admin',
  },
  actions: {
    setUser(u) {
      this.user = u
      localStorage.setItem('guangyanji_user', JSON.stringify(u))
    },
    async fetchMe() {
      try {
        const { data } = await api.me()
        this.setUser(data)
        return data
      } catch {
        this.user = null
        localStorage.removeItem('guangyanji_user')
        return null
      }
    },
    async login(email, password) {
      const { data } = await api.login({ email, password })
      this.setUser(data.user)
    },
    async register(username, email, password) {
      const { data } = await api.register({ username, email, password })
      this.setUser(data.user)
    },
    async logout() {
      try {
        await api.logout()
      } finally {
        this.user = null
        localStorage.removeItem('guangyanji_user')
      }
    },
  },
})
