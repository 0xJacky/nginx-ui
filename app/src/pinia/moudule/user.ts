import { defineStore } from 'pinia'

export const useUserStore = defineStore('user', {
  state: () => ({
    token: '',
    unreadCount: 0,
    secureSessionId: '',
    passkeyRawId: '',
  }),
  getters: {
    isLogin(state): boolean {
      return !!state.token
    },
    passkeyLoginAvailable(state): boolean {
      return !!state.passkeyRawId
    },
  },
  actions: {
    passkeyLogin(rawId: string, token: string) {
      this.passkeyRawId = rawId
      this.login(token)
    },
    login(token: string) {
      this.token = token
    },
    logout() {
      this.token = ''
      this.passkeyRawId = ''
      this.secureSessionId = ''
      this.unreadCount = 0
    },
  },
  persist: true,
})
