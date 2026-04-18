import type { AnalyticNode, Node } from '@/api/node'
import { useDocumentVisibility, useEventListener, useOnline } from '@vueuse/core'
import analytic from '@/api/analytic'
import nodeApi from '@/api/node'
import { useWebSocket } from '@/lib/websocket'

export const useNodeAvailabilityStore = defineStore('nodeAvailability', () => {
  const cacheKey = 'node-availability-snapshot'
  const nodes = ref<Record<string, Partial<AnalyticNode>>>({})
  const websocket = shallowRef<WebSocket | null>(null)
  const isConnected = ref(false)
  const isInitialized = ref(false)
  const lastUpdateTime = ref<string>('')
  const isConnecting = ref(false)
  const nodeList = computed<Partial<AnalyticNode>[]>(() => Object.values(nodes.value))

  function readCachedNodes(): Record<string, Partial<AnalyticNode>> {
    if (typeof window === 'undefined') {
      return {}
    }

    try {
      const raw = window.sessionStorage.getItem(cacheKey)
      return raw ? JSON.parse(raw) as Record<string, Partial<AnalyticNode>> : {}
    }
    catch {
      return {}
    }
  }

  function writeCachedNodes(value: Record<string, Partial<AnalyticNode>>) {
    if (typeof window === 'undefined') {
      return
    }

    try {
      const sanitized = Object.fromEntries(
        Object.entries(value).map(([nodeId, node]) => {
          const { token, ...safeNode } = node
          return [nodeId, safeNode]
        }),
      )
      window.sessionStorage.setItem(cacheKey, JSON.stringify(sanitized))
    }
    catch {
    }
  }

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
    onError() {
      console.warn('Failed to connect to nodes WebSocket endpoint')
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

        writeCachedNodes(nodes.value)

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
      const cachedNodes = readCachedNodes()

      response.data.forEach((node: Node) => {
        const cachedNode = cachedNodes[node.id] ?? {}
        nodeMap[node.id] = {
          ...cachedNode,
          id: node.id,
          name: node.name,
          status: node.status,
          url: node.url,
          token: node.token,
          enabled: true,
        }
      })

      nodes.value = nodeMap
      writeCachedNodes(nodes.value)

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

  // The underlying useWebSocket gives up after ~10 quick retries. That's fine
  // for a transient blip but leaves us stuck for long pauses — backend restart,
  // laptop sleep, flaky VPN. Re-opening the socket whenever the tab becomes
  // visible or the network comes back provides a cheap recovery path without
  // hammering the server while the user isn't looking.
  if (typeof window !== 'undefined') {
    const visibility = useDocumentVisibility()
    const online = useOnline()

    watch(visibility, (value, previous) => {
      if (!isInitialized.value) {
        return
      }
      if (value === 'visible' && previous !== 'visible' && !isConnected.value) {
        connectWebSocket()
      }
    })

    watch(online, (value, previous) => {
      if (!isInitialized.value) {
        return
      }
      if (value && !previous && !isConnected.value) {
        connectWebSocket()
      }
    })
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

  // Auto-cleanup WebSocket on page unload. Using VueUse's useEventListener ties
  // the listener to the setup store's effect scope so HMR/$dispose reliably
  // removes it instead of accumulating duplicates.
  useEventListener(typeof window !== 'undefined' ? window : null, 'beforeunload', () => {
    stopMonitoring()
  })

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
