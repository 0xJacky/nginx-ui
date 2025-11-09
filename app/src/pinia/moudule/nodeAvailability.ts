import type { AnalyticNode, Node } from '@/api/node'
import analytic from '@/api/analytic'
import nodeApi from '@/api/node'
import { useWebSocket } from '@/lib/websocket'

export const useNodeAvailabilityStore = defineStore('nodeAvailability', () => {
  const nodes = ref<Record<string, Partial<AnalyticNode>>>({})
  const websocket = shallowRef<WebSocket | null>(null)
  const isConnected = ref(false)
  const isInitialized = ref(false)
  const lastUpdateTime = ref<string>('')
  const isConnecting = ref(false)
  const nodeList = computed<Partial<AnalyticNode>[]>(() => Object.values(nodes.value))

  const socket = useWebSocket<Record<string, Partial<AnalyticNode>>>(analytic.nodesWebSocketUrl, true, {
    immediate: false,
    autoClose: false,
    onConnected(webSocket) {
      websocket.value = webSocket
      isConnected.value = true
      isConnecting.value = false
    },
    onDisconnected() {
      isConnected.value = false
      isConnecting.value = false
      websocket.value = null
    },
    onError(event) {
      console.warn('Failed to connect to nodes WebSocket endpoint', event)
      isConnected.value = false
      isConnecting.value = false
    },
    onMessage(_, event) {
      try {
        const nodesData = JSON.parse(event.data) as Record<string, Partial<AnalyticNode>>

        Object.keys(nodesData).forEach((nodeIdStr: string) => {
          const nodeId = Number.parseInt(nodeIdStr)
          const nodeData = nodesData[nodeIdStr]

          nodes.value[nodeId] = nodeData
        })

        lastUpdateTime.value = new Date().toISOString()
      }
      catch (error) {
        console.error('Error parsing WebSocket message:', error)
      }
    },
  })

  // Initialize node data from API and WebSocket
  async function initialize() {
    if (isInitialized.value) {
      return
    }

    try {
      // First, load the initial node data from API
      const response = await nodeApi.getList({ enabled: true })
      const nodeMap: Record<string, Partial<AnalyticNode>> = {}

      response.data.forEach((node: Node) => {
        nodeMap[node.id] = {
          id: node.id,
          name: node.name,
          status: node.status,
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
      console.error('Failed to create WebSocket connection:', error)
      isConnecting.value = false
    }
  }

  // Start monitoring (initialize + WebSocket)
  async function startMonitoring() {
    await initialize()
  }

  // Stop monitoring and cleanup
  function stopMonitoring() {
    if (socket.ws.value && socket.ws.value.readyState !== WebSocket.CLOSED) {
      socket.close()
    }

    websocket.value = null
    isConnected.value = false
    isConnecting.value = false
  }

  // Get node status by ID
  function getNodeStatus(nodeId: number): Partial<AnalyticNode> | undefined {
    return nodes.value[nodeId]
  }

  // Get all nodes as array
  function getAllNodes(): Partial<AnalyticNode>[] {
    return Object.values(nodes.value)
  }

  // Get enabled nodes only
  function getEnabledNodes(): Partial<AnalyticNode>[] {
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
    nodeList,
    isConnected: readonly(isConnected),
    isInitialized: readonly(isInitialized),
    lastUpdateTime: readonly(lastUpdateTime),
    isConnecting: readonly(isConnecting),
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
