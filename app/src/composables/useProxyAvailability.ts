// Subscribes the calling scope to the shared proxy availability WebSocket.
import { useProxyAvailabilityStore } from '@/pinia/moudule/proxyAvailability'

/**
 * Keeps upstream availability updates flowing for as long as the calling scope
 * lives, and lets go of them when it is disposed.
 *
 * The socket is reference counted in the store, so unmounting one page never
 * cuts off another page that is still subscribed, while a tab that navigates
 * away from every upstream-aware page does stop holding the connection.
 */
export function useProxyAvailability() {
  const store = useProxyAvailabilityStore()

  const token = store.acquireMonitor()
  // onScopeDispose covers components and standalone effect scopes alike; the
  // silent flag keeps a call from outside a scope (tests, one-off usage) from
  // warning — the store still closes the socket on unload.
  onScopeDispose(() => store.releaseMonitor(token), true)

  return store
}
