import { defineStore } from 'pinia'

export const useUserStore = defineStore('user', {
  state: () => ({
    token: '',
    unreadCount: 0,
  }),
  getters: {
    is_login(state): boolean {
      return !!state.token
    },
  },
  actions: {
    login(token: string) {
      this.token = token
    },
    logout() {
      this.token = ''
    },
  },
  persist: true,
})
