import { beforeEach, describe, expect, mock, test } from 'bun:test'
import { createPinia, defineStore, setActivePinia } from 'pinia'
import { computed, nextTick, ref, watch } from 'vue'

Object.assign(globalThis, {
  defineStore,
  computed,
  ref,
  watch,
  window: { location: { protocol: 'http:' } },
})
mock.module('@vueuse/integrations/useCookies', () => ({
  useCookies: () => ({ set() {}, remove() {}, addChangeListener() {} }),
}))
mock.module('../src/api/2fa', () => ({ default: {} }))
const requests: ((data: { short_token: string }) => void)[] = []
mock.module('../src/api/user', () => ({
  default: { fetchShortToken: () => new Promise(resolve => requests.push(resolve)) },
}))
const { useUserStore } = await import('../src/pinia/moudule/user')

function token(exp: number) {
  return `header.${btoa(JSON.stringify({ exp }))}.signature`
}

beforeEach(() => {
  setActivePinia(createPinia())
  requests.length = 0
})

describe('user session lifecycle', () => {
  test('expiry clears all user credentials and avoids fetching a short token', async () => {
    const user = useUserStore()
    user.shortToken = 'old-short-token'
    user.secureSessionId = 'old-secure-session'
    user.passkeyRawId = 'old-passkey'
    user.login(token(1))
    expect(user.expireSession()).toBe(true)
    await nextTick()
    expect(user.token).toBe('')
    expect(user.shortToken).toBe('')
    expect(user.secureSessionId).toBe('')
    expect(user.passkeyRawId).toBe('')
    expect(user.isLogin).toBe(false)
    expect(requests).toHaveLength(0)
  })

  test('late short-token responses cannot restore logged-out state', async () => {
    const user = useUserStore()
    user.login(token(Date.now() / 1000 + 3600))
    const pending = user.fetchShortToken()
    user.logout()
    requests[0]({ short_token: 'stale' })
    await pending
    expect(user.shortToken).toBe('')
  })

  test('a new login gets its own short token while the old request is still pending', async () => {
    const user = useUserStore()
    user.login(token(Date.now() / 1000 + 3600))
    const oldRequest = user.fetchShortToken()
    user.logout()
    user.login(token(Date.now() / 1000 + 7200))
    const newRequest = user.fetchShortToken()
    expect(requests).toHaveLength(2)
    requests[0]({ short_token: 'old' })
    await oldRequest
    expect(user.shortToken).toBe('')
    const deduplicated = user.fetchShortToken()
    expect(requests).toHaveLength(2)
    requests[1]({ short_token: 'new' })
    await Promise.all([newRequest, deduplicated])
    expect(user.shortToken).toBe('new')
  })
})
