import type { UseWebSocketOptions, UseWebSocketReturn } from '@vueuse/core'
import { useWebSocket as vueUseWebSocket } from '@vueuse/core'
import { storeToRefs } from 'pinia'
import { urlJoin } from '@/lib/helper'
import { useSettingsStore, useUserStore } from '@/pinia'

/**
 * Build WebSocket URL based on environment
 */
export function buildWebSocketUrl(url: string, token: string, shortToken: string, nodeId?: number): string {
  return buildWebSocketUrlWithQuery(url, token, shortToken, undefined, nodeId)
}

export function buildWebSocketUrlWithQuery(
  url: string,
  token: string,
  shortToken: string,
  extraQuery?: Record<string, string | undefined>,
  nodeId?: number,
): string {
  const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:'
  const basePath = import.meta.env.DEV
    ? `${protocol}//${window.location.host}/`
    : `${protocol}//${window.location.host}${urlJoin(window.location.pathname, '/')}`

  const wsUrl = new URL(url, basePath)

  // Use shortToken if available (without base64 encoding), otherwise use regular token (URL-safe base64).
  // URL-safe base64 avoids `+` chars that get decoded as spaces in query strings.
  const longTokenParam = btoa(token).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '')
  wsUrl.searchParams.set('token', shortToken || longTokenParam)

  if (nodeId && nodeId > 0) {
    wsUrl.searchParams.set('x_node_id', String(nodeId))
  }

  if (extraQuery) {
    Object.entries(extraQuery).forEach(([key, value]) => {
      if (value) {
        wsUrl.searchParams.set(key, value)
      }
    })
  }

  return wsUrl.toString()
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
  const wsUrl = buildWebSocketUrlWithQuery(url, token.value, shortToken.value, extraQuery, settings.node.id)

  return vueUseWebSocket<T>(wsUrl, {
    autoReconnect: reconnect
      ? {
          retries: 10,
          delay: 1000,
          onFailed: () => {
            console.warn(`Failed to reconnect to WebSocket after 10 retries: ${url}`)
          },
        }
      : false,
    immediate: true,
    autoClose: true,
    ...options,
  })
}
