import type { SSEvent } from 'sse.js'
import { storeToRefs } from 'pinia'
import { SSE } from 'sse.js'
import { urlJoin } from '@/lib/helper'
import { useSettingsStore, useUserStore } from '@/pinia'

const userStore = useUserStore()
const { token } = storeToRefs(userStore)
const settings = useSettingsStore()

export interface SSEOptions {
  url: string
  // eslint-disable-next-line ts/no-explicit-any
  onMessage?: (data: any) => void
  onError?: () => void
  parseData?: boolean
  reconnectInterval?: number
}

/**
 * Build SSE URL based on environment
 */
function buildSSEUrl(url: string): string {
  // In development mode, connect directly to backend server
  if (import.meta.env.DEV) {
    const proxyTarget = import.meta.env.VITE_PROXY_TARGET || 'http://localhost:9000'

    return urlJoin(proxyTarget, url)
  }

  // In production mode, use relative path
  return urlJoin(window.location.pathname, url)
}

/**
 * SSE Composable
 * Provide the ability to create, manage, and automatically clean up SSE connections
 */
export function useSSE() {
  const sseInstance = shallowRef<SSE>()
  const reconnectTimer = shallowRef<ReturnType<typeof setTimeout>>()
  const isReconnecting = ref(false)
  const currentOptions = shallowRef<SSEOptions>()

  /**
   * Clear reconnect timer
   */
  function clearReconnectTimer() {
    if (reconnectTimer.value) {
      clearTimeout(reconnectTimer.value)
      reconnectTimer.value = undefined
    }
  }

  /**
   * Connect to SSE service
   */
  function connect(options: SSEOptions) {
    const {
      url,
      onMessage,
      onError,
      parseData = true,
      reconnectInterval = 5000,
    } = options

    // Store current options for reconnection
    currentOptions.value = options

    // Clear any existing reconnect timer
    clearReconnectTimer()

    // Disconnect existing connection before creating new one
    if (sseInstance.value) {
      sseInstance.value.close()
    }

    const fullUrl = buildSSEUrl(url)

    const headers: Record<string, string> = {}

    if (token.value) {
      headers.Authorization = token.value
    }

    if (settings.node.id) {
      headers['X-Node-ID'] = settings.node.id.toString()
    }

    const sse = new SSE(fullUrl, {
      headers,
    })

    // Handle messages
    sse.onmessage = (e: SSEvent) => {
      if (!e.data) {
        return
      }

      // Reset reconnecting state on successful message
      isReconnecting.value = false

      try {
        const parsedData = parseData ? JSON.parse(e.data) : e.data
        onMessage?.(parsedData)
      }
      catch (error) {
        console.error('Error parsing SSE message:', error)
      }
    }

    // Handle errors and reconnect
    sse.onerror = () => {
      onError?.()

      // Only attempt reconnection if not already reconnecting and we have current options
      if (!isReconnecting.value && currentOptions.value) {
        isReconnecting.value = true

        // Clear any existing timer before setting new one
        clearReconnectTimer()

        reconnectTimer.value = setTimeout(() => {
          if (currentOptions.value && isReconnecting.value) {
            connect(currentOptions.value)
          }
        }, reconnectInterval)
      }
    }

    sseInstance.value = sse
    return sse
  }

  /**
   * Disconnect SSE connection
   */
  function disconnect() {
    // Clear reconnect timer and state
    clearReconnectTimer()
    isReconnecting.value = false
    currentOptions.value = undefined

    if (sseInstance.value) {
      sseInstance.value.close()
      sseInstance.value = undefined
    }
  }

  // Automatically disconnect when the component is unmounted
  if (getCurrentInstance()) {
    onUnmounted(() => {
      disconnect()
    })
  }

  return {
    connect,
    disconnect,
    sseInstance,
    isReconnecting: readonly(isReconnecting),
  }
}
