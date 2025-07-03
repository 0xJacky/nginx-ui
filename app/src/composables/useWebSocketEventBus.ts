import { v4 as uuidv4 } from 'uuid'
import ws from '@/lib/websocket'

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

class WebSocketEventBus {
  private static instance: WebSocketEventBus
  private ws: WebSocket | null = null
  private subscriptions: Map<string, EventSubscription> = new Map()
  private isConnected = false

  private constructor() {}

  static getInstance(): WebSocketEventBus {
    if (!WebSocketEventBus.instance) {
      WebSocketEventBus.instance = new WebSocketEventBus()
    }
    return WebSocketEventBus.instance
  }

  // Connect to WebSocket
  connect(): void {
    if (this.ws && this.isConnected) {
      return
    }

    // Close existing connection
    if (this.ws) {
      this.ws.close()
    }

    // Use the lib/websocket to create connection with auto-reconnect
    this.ws = ws('/api/events', true) as WebSocket

    this.ws.onopen = () => {
      this.isConnected = true
    }

    this.ws.onmessage = (event: MessageEvent) => {
      try {
        const message: WebSocketMessage = JSON.parse(event.data)
        this.handleMessage(message)
      }
      catch (error) {
        console.error('Error parsing WebSocket message:', error)
      }
    }

    this.ws.onclose = () => {
      this.isConnected = false
    }

    this.ws.onerror = error => {
      console.error('WebSocket error:', error)
      this.isConnected = false
    }
  }

  // Handle incoming WebSocket message
  private handleMessage(message: WebSocketMessage): void {
    // Find all subscriptions for this event
    this.subscriptions.forEach(subscription => {
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

  // Subscribe to an event
  // eslint-disable-next-line ts/no-explicit-any
  subscribe<T = any>(event: string, handler: EventHandler<T>): string {
    const id = uuidv4()

    this.subscriptions.set(id, {
      id,
      event,
      handler,
    })

    // Ensure WebSocket is connected
    if (!this.isConnected) {
      this.connect()
    }

    return id
  }

  // Unsubscribe from an event
  unsubscribe(subscriptionId: string): void {
    this.subscriptions.delete(subscriptionId)
  }

  // Disconnect WebSocket
  disconnect(): void {
    this.isConnected = false
    this.subscriptions.clear()

    if (this.ws) {
      this.ws.close()
      this.ws = null
    }
  }

  // Get connection status
  getConnectionStatus(): boolean {
    return this.isConnected
  }
}

// WebSocket Event Bus Composable
export function useWebSocketEventBus() {
  const eventBus = WebSocketEventBus.getInstance()
  const subscriptionIds = ref<string[]>([])

  // Subscribe to an event
  // eslint-disable-next-line ts/no-explicit-any
  function subscribe<T = any>(event: string, handler: EventHandler<T>): string {
    const id = eventBus.subscribe(event, handler)
    subscriptionIds.value.push(id)
    return id
  }

  // Unsubscribe from an event
  function unsubscribe(subscriptionId: string): void {
    eventBus.unsubscribe(subscriptionId)
    const index = subscriptionIds.value.indexOf(subscriptionId)
    if (index > -1) {
      subscriptionIds.value.splice(index, 1)
    }
  }

  // Connect to WebSocket
  function connect(): void {
    eventBus.connect()
  }

  // Disconnect from WebSocket
  function disconnect(): void {
    eventBus.disconnect()
  }

  // Get connection status
  const isConnected = computed(() => eventBus.getConnectionStatus())

  // Auto cleanup on unmount
  if (getCurrentInstance()) {
    onUnmounted(() => {
      // Unsubscribe all subscriptions for this component
      subscriptionIds.value.forEach(id => {
        eventBus.unsubscribe(id)
      })
      subscriptionIds.value = []
    })
  }

  return {
    subscribe,
    unsubscribe,
    connect,
    disconnect,
    isConnected: readonly(isConnected),
  }
}
