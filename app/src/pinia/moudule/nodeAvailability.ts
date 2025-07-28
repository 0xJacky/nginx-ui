import type ReconnectingWebSocket from 'reconnecting-websocket'
import type { Environment } from '@/api/environment'
import { defineStore } from 'pinia'
import ws from '@/lib/websocket'

export interface NodeStatus {
  id: number
  name: string
  status: boolean
  url?: string
  token?: string
  enabled?: boolean
}

export const useNodeAvailabilityStore = defineStore('nodeAvailability', () => {
  const nodes = ref<Record<number, NodeStatus>>({})
  const websocket = shallowRef<ReconnectingWebSocket | WebSocket>()
  const isConnected = ref(false)
  const isInitialized = ref(false)
  const lastUpdateTime = ref<string>('')

  // Initialize node data from WebSocket
  function initialize() {
    if (isInitialized.value) {
      return
    }

    connectWebSocket()
    isInitialized.value = true
  }

  // Connect to WebSocket for real-time updates
  function connectWebSocket() {
    if (websocket.value && isConnected.value) {
      return
    }

    // Close existing connection if any
    if (websocket.value) {
      websocket.value.close()
    }

    try {
      // Create new WebSocket connection
      const socket = ws('/api/environments/enabled', true)
      websocket.value = socket

      socket.onopen = () => {
        isConnected.value = true
      }

      socket.onmessage = event => {
        try {
          const message = JSON.parse(event.data)

          if (message.event === 'message') {
            const environments: Environment[] = message.data
            const nodeMap: Record<number, NodeStatus> = {}

            environments.forEach(env => {
              nodeMap[env.id] = {
                id: env.id,
                name: env.name,
                status: env.status ?? false,
                url: env.url,
                token: env.token,
                enabled: true,
              }
            })

            nodes.value = nodeMap
            lastUpdateTime.value = new Date().toISOString()
          }
        }
        catch (error) {
          console.error('Error parsing WebSocket message:', error)
        }
      }

      socket.onclose = () => {
        isConnected.value = false
      }

      socket.onerror = error => {
        console.warn('Failed to connect to environments WebSocket endpoint', error)
        isConnected.value = false
      }
    }
    catch (error) {
      console.error('Failed to create WebSocket connection:', error)
    }
  }

  // Start monitoring (initialize + WebSocket)
  function startMonitoring() {
    initialize()
  }

  // Stop monitoring and cleanup
  function stopMonitoring() {
    if (websocket.value) {
      websocket.value.close()
      websocket.value = undefined
      isConnected.value = false
    }
  }

  // Get node status by ID
  function getNodeStatus(nodeId: number): NodeStatus | undefined {
    return nodes.value[nodeId]
  }

  // Get all nodes as array
  function getAllNodes(): NodeStatus[] {
    return Object.values(nodes.value)
  }

  // Get enabled nodes only
  function getEnabledNodes(): NodeStatus[] {
    return Object.values(nodes.value).filter(node => node.enabled)
  }

  // Check if node is online
  function isNodeOnline(nodeId: number): boolean {
    const node = nodes.value[nodeId]
    return node?.status ?? false
  }

  // Get node name by ID
  function getNodeName(nodeId: number): string {
    const node = nodes.value[nodeId]
    return node?.name ?? ''
  }

  // Auto-cleanup WebSocket on page unload
  if (typeof window !== 'undefined') {
    window.addEventListener('beforeunload', () => {
      stopMonitoring()
    })
  }

  return {
    nodes: readonly(nodes),
    isConnected: readonly(isConnected),
    isInitialized: readonly(isInitialized),
    lastUpdateTime: readonly(lastUpdateTime),
    initialize,
    startMonitoring,
    stopMonitoring,
    connectWebSocket,
    getNodeStatus,
    getAllNodes,
    getEnabledNodes,
    isNodeOnline,
    getNodeName,
  }
})
