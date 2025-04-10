import type { SSEvent } from 'sse.js'
import { SSE } from 'sse.js'
import { onUnmounted, shallowRef } from 'vue'

export interface SSEOptions {
  url: string
  token: string
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
    disconnect()

    const {
      url,
      token,
      onMessage,
      onError,
      parseData = true,
      reconnectInterval = 5000,
    } = options

    const sse = new SSE(url, {
      headers: {
        Authorization: token,
      },
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
  onUnmounted(() => {
    disconnect()
  })

  return {
    connect,
    disconnect,
    sseInstance,
  }
}
