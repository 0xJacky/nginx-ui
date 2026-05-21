import type { CookieChangeOptions } from 'universal-cookie'
import type { TwoFAStatus } from '@/api/2fa'
import type { User } from '@/api/user'
import { useCookies } from '@vueuse/integrations/useCookies'
import twoFA from '@/api/2fa'
import userApi from '@/api/user'

export const useUserStore = defineStore('user', () => {
  const cookies = useCookies(['nginx-ui'])

  function getCookieOptions(maxAge: number) {
    return {
      path: '/',
      maxAge,
      sameSite: 'lax' as const,
      secure: window.location.protocol === 'https:',
    }
  }

  const token = ref('')
  const shortToken = ref('')

  let shortTokenRequest: Promise<void> | null = null

  watch(token, v => {
    if (v) {
      cookies.set('token', v, getCookieOptions(86400))
      if (!shortToken.value) {
        void fetchShortToken()
      }
    }
    else {
      cookies.remove('token', { path: '/' })
      shortToken.value = ''
    }
  })

  const secureSessionId = ref('')

  function getEmptyTwoFAStatus(): TwoFAStatus {
    return {
      enabled: false,
      otp_status: false,
      passkey_status: false,
      recovery_codes_generated: false,
      recovery_codes_viewed: false,
      recovery_codes_migration_required: false,
    }
  }

  const twoFAStatus = ref<TwoFAStatus>(getEmptyTwoFAStatus())

  watch(secureSessionId, v => {
    if (v)
      cookies.set('secure_session_id', v, getCookieOptions(60 * 3))
    else
      cookies.remove('secure_session_id', { path: '/' })
  })

  function handleCookieChange({ name, value }: CookieChangeOptions) {
    if (name === 'token')
      token.value = value || ''
    else if (name === 'secure_session_id')
      secureSessionId.value = value || ''
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
    shortToken.value = ''
    passkeyRawId.value = ''
    secureSessionId.value = ''
    unreadCount.value = 0
    info.value = {} as User
    twoFAStatus.value = getEmptyTwoFAStatus()
  }

  async function fetchShortToken() {
    if (!token.value)
      return
    if (shortTokenRequest)
      return shortTokenRequest
    shortTokenRequest = (async () => {
      try {
        const data = await userApi.fetchShortToken()
        shortToken.value = data.short_token
      }
      catch (error) {
        console.error('Failed to fetch short token:', error)
      }
      finally {
        shortTokenRequest = null
      }
    })()

    return shortTokenRequest
  }

  async function getCurrentUser() {
    try {
      const data = await userApi.getCurrentUser()
      info.value = data
      return data
    }
    catch (error) {
      console.error('Failed to get current user:', error)
      throw error
    }
  }

  async function refreshTwoFAStatus() {
    if (!token.value)
      return twoFAStatus.value

    const status = await twoFA.status()
    twoFAStatus.value = status
    return status
  }

  async function updateCurrentUser(userData: Partial<User>) {
    try {
      const response = await userApi.updateCurrentUser(userData as User)
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
      const response = await userApi.updateCurrentUserPassword(data)
      return response.data
    }
    catch (error) {
      console.error('Failed to update password:', error)
      throw error
    }
  }

  async function updateCurrentUserLanguage(language: string) {
    try {
      await userApi.updateCurrentUserLanguage({ language })
      info.value.language = language
    }
    catch (error) {
      console.error('Failed to update language:', error)
      throw error
    }
  }

  // On store initialization, if token exists, fetch a fresh short token
  if (token.value) {
    fetchShortToken()
  }

  return {
    token,
    shortToken,
    unreadCount,
    secureSessionId,
    passkeyRawId,
    info,
    twoFAStatus,
    isLogin,
    passkeyLoginAvailable,
    passkeyLogin,
    login,
    logout,
    fetchShortToken,
    refreshTwoFAStatus,
    getCurrentUser,
    updateCurrentUser,
    updateCurrentUserPassword,
    updateCurrentUserLanguage,
  }
}, {
  persist: {
    pick: ['token', 'secureSessionId', 'passkeyRawId', 'info', 'unreadCount'],
  },
})
