import { createTokenExpiryMonitor } from '@/lib/auth/tokenExpiry'
import { useUserStore } from '@/pinia'

export function useSessionExpiry() {
  const user = useUserStore()
  const router = useRouter()
  let disposed = false
  const monitor = createTokenExpiryMonitor(() => user.token, () => {
    if (!user.expireSession())
      return
    // Let the initial auth guard preserve a deep link instead of racing it
    // with the router's temporary start location (/).
    void router.isReady().then(() => {
      const route = router.currentRoute.value
      if (!disposed && !user.isLogin && !route.meta.noAuth)
        void router.replace({ path: '/login', query: { next: route.fullPath } })
    })
  })

  watch(() => user.token, monitor.check, { immediate: true, flush: 'sync' })

  // Timers may be suspended while the device sleeps or the tab is backgrounded.
  window.addEventListener('focus', monitor.check)
  window.addEventListener('pageshow', monitor.check)
  document.addEventListener('visibilitychange', monitor.check)
  onScopeDispose(() => {
    disposed = true
    monitor.stop()
    window.removeEventListener('focus', monitor.check)
    window.removeEventListener('pageshow', monitor.check)
    document.removeEventListener('visibilitychange', monitor.check)
  })
}
