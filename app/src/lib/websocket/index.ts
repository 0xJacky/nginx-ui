import { storeToRefs } from 'pinia'
import ReconnectingWebSocket from 'reconnecting-websocket'
import { urlJoin } from '@/lib/helper'
import { useSettingsStore, useUserStore } from '@/pinia'

/**
 * Build WebSocket URL based on environment
 */
function buildWebSocketUrl(url: string, token: string, nodeId?: number): string {
  const node_id = nodeId && nodeId > 0 ? `&x_node_id=${nodeId}` : ''

  // In development mode, connect directly to backend server
  if (import.meta.env.DEV) {
    const proxyTarget = import.meta.env.VITE_PROXY_TARGET || 'http://localhost:9000'
    const wsTarget = proxyTarget.replace(/^https?:/, location.protocol === 'https:' ? 'wss:' : 'ws:')
    return urlJoin(wsTarget, url, `?token=${btoa(token)}`, node_id)
  }

  // In production mode, use current host
  const protocol = location.protocol === 'https:' ? 'wss://' : 'ws://'
  return urlJoin(protocol + window.location.host, window.location.pathname, url, `?token=${btoa(token)}`, node_id)
}

function ws(url: string, reconnect: boolean = true): ReconnectingWebSocket | WebSocket {
  const user = useUserStore()
  const settings = useSettingsStore()
  const { token } = storeToRefs(user)

  const _url = buildWebSocketUrl(url, token.value, settings.environment.id)

  if (reconnect)
    return new ReconnectingWebSocket(_url, undefined, { maxRetries: 10 })

  return new WebSocket(_url)
}

export default ws
