import type { SSEvent } from 'sse.js'
import { urlJoin } from '@/lib/helper'
import { useSettingsStore, useUserStore } from '@/pinia'
import { storeToRefs } from 'pinia'
import { SSE } from 'sse.js'

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
 * SSE Composable
 * Provide the ability to create, manage, and automatically clean up SSE connections
 */
export function useSSE() {
  const sseInstance = shallowRef<SSE>()

  /**
   * Connect to SSE service
   */
  function connect(options: SSEOptions) {
    if (!token.value) {
      return
    }

    const {
      url,
      onMessage,
      onError,
      parseData = true,
      reconnectInterval = 5000,
    } = options

    const fullUrl = urlJoin(window.location.pathname, url)

    const headers = {
      Authorization: token.value,
    }

    if (settings.environment.id) {
      headers['X-Node-ID'] = settings.environment.id.toString()
    }

    const sse = new SSE(fullUrl, {
      headers,
    })

    // Handle messages
    sse.onmessage = (e: SSEvent) => {
      if (!e.data) {
        return
      }

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

      // Reconnect logic
      setTimeout(() => {
        connect(options)
      }, reconnectInterval)
    }

    sseInstance.value = sse
    return sse
  }

  /**
   * Disconnect SSE connection
   */
  function disconnect() {
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
  }
}
