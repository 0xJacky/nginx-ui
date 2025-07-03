import { storeToRefs } from 'pinia'
import ReconnectingWebSocket from 'reconnecting-websocket'
import { urlJoin } from '@/lib/helper'
import { useSettingsStore, useUserStore } from '@/pinia'

/**
 * Build WebSocket URL based on environment
 */
function buildWebSocketUrl(url: string, token: string, shortToken: string, nodeId?: number): string {
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

function ws(url: string, reconnect: boolean = true): ReconnectingWebSocket | WebSocket {
  const user = useUserStore()
  const settings = useSettingsStore()
  const { token, shortToken } = storeToRefs(user)

  const _url = buildWebSocketUrl(url, token.value, shortToken.value, settings.environment.id)

  if (reconnect)
    return new ReconnectingWebSocket(_url, undefined, { maxRetries: 10 })

  return new WebSocket(_url)
}

export default ws
