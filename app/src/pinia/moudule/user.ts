import type { CookieChangeOptions } from 'universal-cookie'
import type { TwoFAStatus } from '@/api/2fa'
import type { User } from '@/api/user'
import { useCookies } from '@vueuse/integrations/useCookies'
import twoFA from '@/api/2fa'
import userApi from '@/api/user'

// Matches the release-build two-factor session window on the backend.
const defaultSecureSessionTTL = 60 * 10

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
      if (!shortToken.value) {
        void fetchShortToken()
      }
    }
    else {
      shortToken.value = ''
    }
  })

  const secureSessionId = ref('')
  // Seconds the backend keeps a verified two-factor session. Reported when the
  // session is created so the cookie does not expire before the server side.
  const secureSessionTTL = ref(defaultSecureSessionTTL)

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

  // Set while mirroring a cookie another tab wrote, so this tab does not write
  // the shared cookie back with its own, possibly shorter, TTL.
  let syncingFromCookie = false

  watch([secureSessionId, secureSessionTTL], ([id, ttl]) => {
    if (syncingFromCookie) {
      syncingFromCookie = false
      return
    }
    if (id)
      cookies.set('secure_session_id', id, getCookieOptions(ttl || defaultSecureSessionTTL))
    else
      cookies.remove('secure_session_id', { path: '/' })
  })

  function setSecureSession(id: string, ttl?: number) {
    secureSessionTTL.value = ttl && ttl > 0 ? ttl : defaultSecureSessionTTL
    secureSessionId.value = id
  }

  function handleCookieChange({ name, value }: CookieChangeOptions) {
    if (name !== 'secure_session_id')
      return
    const next = value || ''
    if (next === secureSessionId.value)
      return
    syncingFromCookie = true
    secureSessionId.value = next
  }

  // Remove the legacy ambient JWT cookie. Authentication state is persisted by
  // Pinia and sent explicitly through the Authorization header.
  cookies.remove('token', { path: '/' })
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
    secureSessionTTL.value = defaultSecureSessionTTL
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

    try {
      const status = await twoFA.status()
      twoFAStatus.value = status
      return status
    }
    catch (error) {
      console.error('Failed to refresh 2FA status:', error)
      return twoFAStatus.value
    }
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
    secureSessionTTL,
    setSecureSession,
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
    pick: ['token', 'secureSessionId', 'secureSessionTTL', 'passkeyRawId', 'info', 'unreadCount'],
  },
})
