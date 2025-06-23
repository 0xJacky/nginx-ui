import type { CookieChangeOptions } from 'universal-cookie'
import type { User } from '@/api/user'
import { useCookies } from '@vueuse/integrations/useCookies'
import { defineStore } from 'pinia'
import user from '@/api/user'

export const useUserStore = defineStore('user', () => {
  const cookies = useCookies(['nginx-ui'])

  const token = ref('')

  watch(token, v => {
    cookies.set('token', v, { maxAge: 86400 })
  })

  const secureSessionId = ref('')

  watch(secureSessionId, v => {
    cookies.set('secure_session_id', v, { maxAge: 60 * 3 })
  })

  function handleCookieChange({ name, value }: CookieChangeOptions) {
    if (name === 'token')
      token.value = value
    else if (name === 'secure_session_id')
      secureSessionId.value = value
  }

  cookies.addChangeListener(handleCookieChange)

  const passkeyRawId = ref('')
  const info = ref<User>({} as User)

  const unreadCount = ref(0)
  const isLogin = computed(() => !!token.value)
  const passkeyLoginAvailable = computed(() => !!passkeyRawId.value)

  function passkeyLogin(rawId: string, tokenValue: string) {
    passkeyRawId.value = rawId
    login(tokenValue)
  }

  function login(tokenValue: string) {
    token.value = tokenValue
  }

  function logout() {
    token.value = ''
    passkeyRawId.value = ''
    secureSessionId.value = ''
    unreadCount.value = 0
    info.value = {} as User
  }

  async function getCurrentUser() {
    try {
      const data = await user.getCurrentUser()
      info.value = data
      return data
    }
    catch (error) {
      console.error('Failed to get current user:', error)
      throw error
    }
  }

  async function updateCurrentUser(userData: Partial<User>) {
    try {
      const response = await user.updateCurrentUser(userData as User)
      info.value = { ...info.value, ...userData }
      return response.data
    }
    catch (error) {
      console.error('Failed to update current user:', error)
      throw error
    }
  }

  async function updateCurrentUserPassword(data: { old_password: string, new_password: string }) {
    try {
      const response = await user.updateCurrentUserPassword(data)
      return response.data
    }
    catch (error) {
      console.error('Failed to update password:', error)
      throw error
    }
  }

  async function updateCurrentUserLanguage(language: string) {
    try {
      await user.updateCurrentUserLanguage({ language })
      info.value.language = language
    }
    catch (error) {
      console.error('Failed to update language:', error)
      throw error
    }
  }

  return {
    token,
    unreadCount,
    secureSessionId,
    passkeyRawId,
    info,
    isLogin,
    passkeyLoginAvailable,
    passkeyLogin,
    login,
    logout,
    getCurrentUser,
    updateCurrentUser,
    updateCurrentUserPassword,
    updateCurrentUserLanguage,
  }
}, {
  persist: true,
})
