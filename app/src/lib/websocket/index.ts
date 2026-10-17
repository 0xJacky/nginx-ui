import type { UseWebSocketOptions, UseWebSocketReturn } from '@vueuse/core'
import { useWebSocket as vueUseWebSocket } from '@vueuse/core'
import { storeToRefs } from 'pinia'
import { useSettingsStore, useUserStore } from '@/pinia'
import { buildWebSocketUrlWithQuery } from './url'

export { buildWebSocketUrl, buildWebSocketUrlWithQuery } from './url'

function resolveAutoReconnect(url: string, reconnect: boolean): UseWebSocketOptions['autoReconnect'] {
  return reconnect
    ? {
        retries: 10,
        delay: 1000,
        onFailed: () => {
          console.warn(`Failed to reconnect to WebSocket after 10 retries: ${url}`)
        },
      }
    : false
}

/**
 * Create a WebSocket connection using VueUse
 * @param url - The WebSocket endpoint URL
 * @param reconnect - Whether to enable auto-reconnect (default: true)
 * @param options - Additional VueUse WebSocket options
 */
// eslint-disable-next-line ts/no-explicit-any
export function useWebSocket<T = any>(
  url: string,
  reconnect: boolean = true,
  options?: Omit<UseWebSocketOptions, 'autoReconnect'>,
  extraQuery?: Record<string, string | undefined>,
  /** Connects to this server even while a remote node is selected. */
  skipNodeProxy = false,
): UseWebSocketReturn<T> {
  const userStore = useUserStore()
  const settings = useSettingsStore()
  const { token, shortToken } = storeToRefs(userStore)

  // Snapshot the URL at call time — must NOT be reactive to avoid tearing down
  // in-flight connections (e.g. terminal, log tail) when shortToken arrives later.
  // When shortToken is empty we fall back to the URL-safe base64 long token,
  // which the backend still accepts. We deliberately do NOT trigger
  // fetchShortToken() here: /token/short can return 403 if the secure-session
  // cookie is stale, and the global HTTP interceptor turns any 403 into a
  // forced logout — which would kick out otherwise-valid sessions on any
  // WebSocket-backed page. Short-token refresh is handled by the user store's
  // token watcher (see app/src/pinia/moudule/user.ts).
  const wsUrl = buildWebSocketUrlWithQuery(url, token.value, shortToken.value, extraQuery, skipNodeProxy ? undefined : settings.node.id)

  return vueUseWebSocket<T>(wsUrl, {
    autoReconnect: resolveAutoReconnect(url, reconnect),
    immediate: true,
    autoClose: true,
    ...options,
  })
}

/**
 * Create a WebSocket owned by a Pinia store (or anything else that outlives a
 * login session).
 *
 * useWebSocket() bakes the credentials into the URL once, which is right for a
 * page-scoped socket that is recreated on every mount. A store is set up once
 * per tab, though, and logging out and back in happens without a reload, so a
 * snapshotted URL keeps dialling with the previous session's credential: a
 * long token is deleted on logout and every reconnect is rejected, while a
 * short token still authenticates as the session that already ended. Here the
 * URL is resolved on each connection attempt instead — the first open and
 * every autoReconnect retry — so reconnects always carry the current session.
 *
 * The URL is deliberately not watched (autoConnect is off): a changing token
 * must not tear down a healthy connection, only the next dial picks it up.
 * The socket starts closed and stays open until close() is called; lifecycle
 * belongs to the owner.
 */
// eslint-disable-next-line ts/no-explicit-any
export function useStoreWebSocket<T = any>(
  url: string,
  reconnect: boolean = true,
  options?: Omit<UseWebSocketOptions, 'autoReconnect' | 'autoConnect' | 'immediate' | 'autoClose'>,
  extraQuery?: Record<string, string | undefined>,
): UseWebSocketReturn<T> {
  const userStore = useUserStore()
  const settings = useSettingsStore()
  const { token, shortToken } = storeToRefs(userStore)

  return vueUseWebSocket<T>(
    () => buildWebSocketUrlWithQuery(url, token.value, shortToken.value, extraQuery, settings.node.id),
    {
      ...options,
      autoReconnect: resolveAutoReconnect(url, reconnect),
      autoConnect: false,
      immediate: false,
      autoClose: false,
    },
  )
}
