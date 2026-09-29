import { buildWebSocketUrl } from '@/lib/websocket/url'

/** Session credentials and selected node a host WebSocket carries. */
export interface WebSocketCredentials {
  token: string
  shortToken: string
  nodeId: number
}

/**
 * Absolute ws or wss URL of a path under a plugin's http capability, built
 * like the host's own WebSocket URLs: session credentials in the query string
 * and the selected node for cluster proxying.
 */
export function buildPluginWebSocketUrl(pluginId: string, path: string, credentials: WebSocketCredentials): string {
  const endpoint = `api/plugins/${encodeURIComponent(pluginId)}/http/${path.replace(/^\/+/, '')}`

  return buildWebSocketUrl(endpoint, credentials.token, credentials.shortToken, credentials.nodeId)
}
