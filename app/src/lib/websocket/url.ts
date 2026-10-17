/**
 * Pure URL builders for WebSocket endpoints. They read no store, so they can
 * be used and tested without the application state.
 */

function normalizeWebSocketEndpoint(url: string): string {
  if (/^[a-z][a-z\d+\-.]*:\/\//i.test(url) || url.startsWith('//')) {
    return url
  }

  return url.replace(/^\/+/, '')
}

function buildWebSocketBaseUrl(protocol: string): string {
  if (import.meta.env.DEV) {
    return `${protocol}//${window.location.host}/`
  }

  const baseUrl = new URL('./', window.location.href)
  baseUrl.protocol = protocol
  baseUrl.search = ''
  baseUrl.hash = ''

  return baseUrl.toString()
}

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
  const basePath = buildWebSocketBaseUrl(protocol)

  const wsUrl = new URL(normalizeWebSocketEndpoint(url), basePath)

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
