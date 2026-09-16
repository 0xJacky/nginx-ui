import type { SubscriptionToken } from '@/lib/websocket/sharedConnection'
import { v4 as uuidv4 } from 'uuid'
import { useStoreWebSocket } from '@/lib/websocket'
import { createSharedConnection } from '@/lib/websocket/sharedConnection'
import { useConnectionSupervisor } from '@/lib/websocket/useConnectionSupervisor'

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

// Keep the bus open briefly after the last subscriber leaves, so a page that
// unsubscribes on its way out does not bounce the connection for the next one.
const IDLE_LINGER_MS = 15_000
const WATCHDOG_INTERVAL_MS = 15_000

export const useWebSocketEventBusStore = defineStore('websocketEventBus', () => {
  // State
  const ws = ref<WebSocket | null>(null)
  const subscriptions = ref<Map<string, EventSubscription>>(new Map())
  // Each subscription holds a reference on the shared connection.
  const connectionTokens = new Map<string, SubscriptionToken>()
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
  const socket = useStoreWebSocket<WebSocketMessage>('/api/events', true, {
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

    // Trust the socket, not the flag: a stale isConnecting would only block
    // recovery, and VueUse creates the WebSocket synchronously in open().
    if (readyState === WebSocket.CONNECTING) {
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

  // Events arrive whenever something happens, so silence is normal and only a
  // dead socket is reconnected (the server pings to catch half-open clients).
  const supervisor = useConnectionSupervisor({
    socket,
    connect,
    staleAfterMs: Number.POSITIVE_INFINITY,
    intervalMs: WATCHDOG_INTERVAL_MS,
  })

  function closeSocket() {
    // Unconditional: between autoReconnect retries the socket is already
    // CLOSED, and skipping close() would let the queued retry reopen it.
    socket.close()
    isConnected.value = false
    isConnecting.value = false
    ws.value = null
  }

  // The bus is open while anything is subscribed, not from the first subscribe
  // until the tab closes. Header components subscribe for the whole session,
  // so in practice this closes it on logout and reopens it on the next login.
  const connection = createSharedConnection({
    open: () => {
      connect()
      supervisor.start()
    },
    close: () => {
      supervisor.stop()
      closeSocket()
    },
    lingerMs: IDLE_LINGER_MS,
  })

  // Subscribe to an event
  // eslint-disable-next-line ts/no-explicit-any
  function subscribe<T = any>(event: string, handler: EventHandler<T>): string {
    const id = uuidv4()

    subscriptions.value.set(id, {
      id,
      event,
      handler,
    })

    // Holding a reference keeps the WebSocket connected (and reconnects it if
    // it had died) for as long as this subscription exists.
    connectionTokens.set(id, connection.acquire())

    return id
  }

  // Unsubscribe from an event
  function unsubscribe(subscriptionId: string): void {
    subscriptions.value.delete(subscriptionId)

    const token = connectionTokens.get(subscriptionId)
    if (token) {
      connectionTokens.delete(subscriptionId)
      connection.release(token)
    }
  }

  // Disconnect WebSocket immediately and drop every subscription (logout)
  function disconnect(): void {
    subscriptions.value.clear()
    connectionTokens.clear()
    connection.shutdown()
  }

  // Get all subscriptions for debugging
  const allSubscriptions = computed(() => [...subscriptions.value.values()])

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
