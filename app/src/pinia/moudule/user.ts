import { defineStore } from 'pinia'
import { useCookies } from '@vueuse/integrations/useCookies'
import type { CookieChangeOptions } from 'universal-cookie'

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
  }

  return {
    token,
    unreadCount,
    secureSessionId,
    passkeyRawId,
    isLogin,
    passkeyLoginAvailable,
    passkeyLogin,
    login,
    logout,
  }
}, {
  persist: true,
})
