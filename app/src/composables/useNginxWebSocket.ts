import ws from '@/lib/websocket'
import type ReconnectingWebSocket from 'reconnecting-websocket'

export interface NginxWebSocketOptions {
  url: string
  onMessage?: (data: any) => void
  onError?: () => void
  reconnectInterval?: number
}

/**
 * Nginx WebSocket Composable
 * Provide the ability to create, manage, and automatically clean up WebSocket connections for Nginx performance monitoring
 */
export function useNginxWebSocket() {
  const wsInstance = shallowRef<ReconnectingWebSocket | WebSocket>()
  const isConnected = ref(false)
  const isReconnecting = ref(false)
  const currentOptions = shallowRef<NginxWebSocketOptions>()

  /**
   * Connect to WebSocket service
   */
  function connect(options: NginxWebSocketOptions) {
    const {
      url,
      onMessage,
      onError,
    } = options

    // Store current options for reconnection
    currentOptions.value = options

    // Disconnect existing connection before creating new one
    if (wsInstance.value) {
      disconnect()
    }

    try {
      const wsConnection = ws(url, true) as ReconnectingWebSocket

      // Handle connection open
      wsConnection.onopen = () => {
        isConnected.value = true
        isReconnecting.value = false
        console.log('WebSocket connected')
      }

      // Handle messages
      wsConnection.onmessage = (event) => {
        if (!event.data) {
          return
        }

        // Reset reconnecting state on successful message
        isReconnecting.value = false

        try {
          const parsedData = JSON.parse(event.data)
          onMessage?.(parsedData)
        }
        catch (error) {
          console.error('Error parsing WebSocket message:', error)
        }
      }

      // Handle errors and connection close
      wsConnection.onerror = (error) => {
        console.error('WebSocket error:', error)
        isConnected.value = false
        isReconnecting.value = true
        onError?.()
      }

      wsConnection.onclose = () => {
        isConnected.value = false
        console.log('WebSocket disconnected')
      }

      wsInstance.value = wsConnection
      return wsConnection
    }
    catch (error) {
      console.error('Failed to create WebSocket connection:', error)
      onError?.()
    }
  }

  /**
   * Disconnect WebSocket connection
   */
  function disconnect() {
    if (wsInstance.value) {
      wsInstance.value.close()
      wsInstance.value = undefined
    }
    isConnected.value = false
    isReconnecting.value = false
    currentOptions.value = undefined
  }

  /**
   * Send message to WebSocket
   */
  function send(data: any) {
    if (wsInstance.value && isConnected.value) {
      wsInstance.value.send(JSON.stringify(data))
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
    send,
    wsInstance,
    isConnected: readonly(isConnected),
    isReconnecting: readonly(isReconnecting),
  }
} 