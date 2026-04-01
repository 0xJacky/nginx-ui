import type { UseWebSocketOptions, UseWebSocketReturn } from '@vueuse/core'
import { useWebSocket as vueUseWebSocket } from '@vueuse/core'
import { storeToRefs } from 'pinia'
import { urlJoin } from '@/lib/helper'
import { useSettingsStore, useUserStore } from '@/pinia'

/**
 * Build WebSocket URL based on environment
 */
export function buildWebSocketUrl(url: string, token: string, shortToken: string, nodeId?: number): string {
  const node_id = nodeId && nodeId > 0 ? `&x_node_id=${nodeId}` : ''

  // Use shortToken if available (without base64 encoding), otherwise use regular token (with base64 encoding)
  const authParam = shortToken ? `token=${shortToken}` : `token=${btoa(token)}`

  // In development mode, connect directly to backend server
  if (import.meta.env.DEV) {
    const proxyTarget = import.meta.env.VITE_PROXY_TARGET || 'http://localhost:9000'
    const wsTarget = proxyTarget.replace(/^https?:/, location.protocol === 'https:' ? 'wss:' : 'ws:')
    return urlJoin(wsTarget, url, `?${authParam}`, node_id)
  }

  // In production mode, use current host
  const protocol = location.protocol === 'https:' ? 'wss://' : 'ws://'
  return urlJoin(protocol + window.location.host, window.location.pathname, url, `?${authParam}`, node_id)
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
): UseWebSocketReturn<T> {
  const userStore = useUserStore()
  const settings = useSettingsStore()
  const { token, shortToken } = storeToRefs(userStore)

  // Reactively rebuild the URL when shortToken changes (e.g. after fetch completes)
  const wsUrl = computed(() =>
    buildWebSocketUrl(url, token.value, shortToken.value, settings.node.id),
  )

  // If short token is not yet available, trigger a fetch (non-blocking)
  if (!shortToken.value && token.value) {
    userStore.fetchShortToken()
  }

  return vueUseWebSocket<T>(wsUrl, {
    autoReconnect: reconnect
      ? {
          retries: 10,
          delay: 1000,
          onFailed: () => {
            userStore.fetchShortToken()
            console.warn(`Failed to reconnect to WebSocket after 10 retries: ${url}`)
          },
        }
      : false,
    immediate: true,
    autoClose: true,
    ...options,
  })
}
