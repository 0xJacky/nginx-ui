import { describe, expect, mock, test } from 'bun:test'
import { effectScope, nextTick, onScopeDispose, reactive, ref, watch } from 'vue'

interface SessionFixture {
  token: string
  isLogin: boolean
  expireSession: () => boolean
}

let activeUser: SessionFixture
mock.module('../src/pinia', () => ({ useUserStore: () => activeUser }))
const { useSessionExpiry } = await import('../src/composables/useSessionExpiry')

function fixture() {
  let resolveReady!: () => void
  const ready = new Promise<void>(resolve => {
    resolveReady = resolve
  })
  const router = {
    currentRoute: ref({ fullPath: '/', meta: { noAuth: false } }),
    isReady: () => ready,
    replace: mock(() => Promise.resolve()),
  }
  activeUser = reactive({
    token: `header.${btoa(JSON.stringify({ exp: 1 }))}.signature`,
    get isLogin() { return !!this.token },
    expireSession() {
      if (!this.token)
        return false
      this.token = ''
      return true
    },
  })
  Object.assign(globalThis, {
    watch,
    onScopeDispose,
    useRouter: () => router,
    window: Object.assign(new EventTarget(), { location: { protocol: 'http:' } }),
    document: new EventTarget(),
  })
  const scope = effectScope()
  scope.run(useSessionExpiry)
  return { router, scope, resolveReady, user: activeUser }
}

describe('session expiry navigation', () => {
  test('does not replace the initial auth guard deep-link redirect', async () => {
    const f = fixture()
    expect(f.user.isLogin).toBe(false)
    expect(f.router.replace).not.toHaveBeenCalled()
    f.router.currentRoute.value = { fullPath: '/login?next=/dashboard/nginx', meta: { noAuth: true } }
    f.resolveReady()
    await nextTick()
    expect(f.router.replace).not.toHaveBeenCalled()
    f.scope.stop()
  })

  test('redirects an expired protected page with its return path', async () => {
    const f = fixture()
    f.router.currentRoute.value = { fullPath: '/dashboard/nginx', meta: { noAuth: false } }
    f.resolveReady()
    await nextTick()
    expect(f.router.replace).toHaveBeenCalledWith({ path: '/login', query: { next: '/dashboard/nginx' } })
    f.scope.stop()
  })

  test('does not redirect a replacement login when initialization completes', async () => {
    const f = fixture()
    f.user.token = `header.${btoa(JSON.stringify({ exp: Date.now() / 1000 + 3600 }))}.signature`
    f.resolveReady()
    await nextTick()
    expect(f.router.replace).not.toHaveBeenCalled()
    f.scope.stop()
  })

  test('does not navigate after the owning scope has been disposed', async () => {
    const f = fixture()
    f.scope.stop()
    f.resolveReady()
    await nextTick()
    expect(f.router.replace).not.toHaveBeenCalled()
  })
})
