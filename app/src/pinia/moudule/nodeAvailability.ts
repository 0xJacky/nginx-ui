import type ReconnectingWebSocket from 'reconnecting-websocket'
import type { Node } from '@/api/node'
import { defineStore } from 'pinia'
import nodeApi from '@/api/node'
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

  // Initialize node data from API and WebSocket
  async function initialize() {
    if (isInitialized.value) {
      return
    }

    try {
      // First, load the initial node data from API
      const response = await nodeApi.getList({ enabled: true })
      const nodeMap: Record<number, NodeStatus> = {}

      response.data.forEach((node: Node) => {
        nodeMap[node.id] = {
          id: node.id,
          name: node.name,
          status: node.status ?? false,
          url: node.url,
          token: node.token,
          enabled: true,
        }
      })

      nodes.value = nodeMap

      // Then connect WebSocket for real-time updates
      connectWebSocket()
      isInitialized.value = true
    }
    catch (error) {
      console.error('Failed to initialize node data:', error)
      // Still try to connect WebSocket even if API call fails
      connectWebSocket()
      isInitialized.value = true
    }
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
      const socket = ws('/api/analytic/nodes', true)
      websocket.value = socket

      socket.onopen = () => {
        isConnected.value = true
      }

      socket.onmessage = event => {
        try {
          const nodesData = JSON.parse(event.data)

          // The /api/analytic/nodes endpoint returns an object with node IDs as keys
          // Update existing nodes' status or create new ones if not exist
          Object.keys(nodesData).forEach((nodeIdStr: string) => {
            const nodeId = Number.parseInt(nodeIdStr)
            const nodeData = nodesData[nodeIdStr]

            // Update existing node or create new one
            const existingNode = nodes.value[nodeId]
            if (existingNode) {
              // Update status for existing node
              existingNode.status = nodeData.status ?? false
            }
            else {
              // Create new node entry (this should be initialized from API call)
              nodes.value[nodeId] = {
                id: nodeId,
                name: nodeData.name || `Node ${nodeId}`,
                status: nodeData.status ?? false,
                url: nodeData.url,
                token: nodeData.token,
                enabled: true,
              }
            }
          })

          lastUpdateTime.value = new Date().toISOString()
        }
        catch (error) {
          console.error('Error parsing WebSocket message:', error)
        }
      }

      socket.onclose = () => {
        isConnected.value = false
      }

      socket.onerror = error => {
        console.warn('Failed to connect to nodes WebSocket endpoint', error)
        isConnected.value = false
      }
    }
    catch (error) {
      console.error('Failed to create WebSocket connection:', error)
    }
  }

  // Start monitoring (initialize + WebSocket)
  async function startMonitoring() {
    await initialize()
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
