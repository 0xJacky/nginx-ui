import { v4 as uuidv4 } from 'uuid'
import { useWebSocket } from '@/lib/websocket'

export interface WebSocketMessage {
  event: string
  // eslint-disable-next-line ts/no-explicit-any
  data: any
}

// eslint-disable-next-line ts/no-explicit-any
export type EventHandler<T = any> = (data: T) => void

export interface EventSubscription {
  id: string
  event: string
  handler: EventHandler
}

export const useWebSocketEventBusStore = defineStore('websocketEventBus', () => {
  // State
  const ws = ref<WebSocket | null>(null)
  const subscriptions = ref<Map<string, EventSubscription>>(new Map())
  const isConnected = ref(false)
  const isConnecting = ref(false)

  // Handle incoming WebSocket message
  function handleMessage(message: WebSocketMessage): void {
    // Find all subscriptions for this event
    subscriptions.value.forEach(subscription => {
      if (subscription.event === message.event) {
        try {
          subscription.handler(message.data)
        }
        catch (error) {
          console.error(`Error handling event ${message.event}:`, error)
        }
      }
    })
  }

  // Connect to WebSocket
  const socket = useWebSocket<WebSocketMessage>('/api/events', true, {
    immediate: false,
    autoClose: false,
    onConnected(webSocket) {
      ws.value = webSocket
      isConnected.value = true
      isConnecting.value = false
    },
    onDisconnected() {
      isConnected.value = false
      isConnecting.value = false
      ws.value = null
    },
    onError(event) {
      console.error('WebSocket error:', event)
      isConnected.value = false
      isConnecting.value = false
    },
    onMessage(_, event) {
      try {
        const message: WebSocketMessage = JSON.parse(event.data)
        handleMessage(message)
      }
      catch (error) {
        console.error('Error parsing WebSocket message:', error)
      }
    },
  })

  function connect(): void {
    const readyState = socket.ws.value?.readyState

    if (readyState === WebSocket.OPEN) {
      isConnected.value = true
      isConnecting.value = false
      return
    }

    if (readyState === WebSocket.CONNECTING || isConnecting.value) {
      isConnecting.value = true
      return
    }

    isConnecting.value = true

    try {
      socket.open()
    }
    catch (error) {
      console.error('Failed to initiate WebSocket connection:', error)
      isConnecting.value = false
    }
  }

  // Subscribe to an event
  // eslint-disable-next-line ts/no-explicit-any
  function subscribe<T = any>(event: string, handler: EventHandler<T>): string {
    const id = uuidv4()

    subscriptions.value.set(id, {
      id,
      event,
      handler,
    })

    // Ensure WebSocket is connected
    if (!isConnected.value) {
      connect()
    }

    return id
  }

  // Unsubscribe from an event
  function unsubscribe(subscriptionId: string): void {
    subscriptions.value.delete(subscriptionId)
  }

  // Disconnect WebSocket
  function disconnect(): void {
    isConnected.value = false
    isConnecting.value = false
    subscriptions.value.clear()

    if (socket.ws.value && socket.ws.value.readyState !== WebSocket.CLOSED) {
      socket.close()
    }

    ws.value = null
  }

  // Get all subscriptions for debugging
  const allSubscriptions = computed(() => Array.from(subscriptions.value.values()))

  return {
    // State (readonly)
    isConnected: readonly(isConnected),
    allSubscriptions,
    isConnecting: readonly(isConnecting),

    // Actions
    connect,
    disconnect,
    subscribe,
    unsubscribe,
  }
})
