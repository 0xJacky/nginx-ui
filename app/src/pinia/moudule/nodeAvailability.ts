import type { AnalyticNode, Node } from '@/api/node'
import { useEventListener } from '@vueuse/core'
import analytic from '@/api/analytic'
import nodeApi from '@/api/node'
import { useStoreWebSocket } from '@/lib/websocket'
import { useConnectionSupervisor } from '@/lib/websocket/useConnectionSupervisor'

// The backend pushes the node map every 10s, so this much silence means the
// connection is gone even if readyState still reads OPEN.
const STALE_MESSAGE_MS = 30_000
const WATCHDOG_INTERVAL_MS = 15_000

export const useNodeAvailabilityStore = defineStore('nodeAvailability', () => {
  const cacheKey = 'node-availability-snapshot'
  const nodes = ref<Record<string, Partial<AnalyticNode>>>({})
  const websocket = shallowRef<WebSocket | null>(null)
  const isConnected = ref(false)
  const isInitialized = ref(false)
  const lastUpdateTime = ref<string>('')
  const isConnecting = ref(false)
  // Whether BaseLayout currently wants live node data (logged-in session).
  let isMonitoring = false
  let initializing: Promise<void> | undefined
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
          const { token: _legacyToken, ...safeNode } = node as Partial<AnalyticNode> & { token?: string }
          return [nodeId, safeNode]
        }),
      )
      window.sessionStorage.setItem(cacheKey, JSON.stringify(sanitized))
    }
    catch {
    }
  }

  const socket = useStoreWebSocket<Record<string, Partial<AnalyticNode>>>(analytic.nodesWebSocketUrl, true, {
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

        nodes.value = nodesData

        writeCachedNodes(nodes.value)

        lastUpdateTime.value = new Date().toISOString()
      }
      catch (error) {
        console.error('Error parsing WebSocket message:', error)
      }
    },
  })

  // Load the node list from the API once. Connecting is startMonitoring()'s job:
  // this runs only for the first session in a tab, while the socket has to be
  // reopened after every logout and login.
  async function initialize() {
    if (isInitialized.value) {
      return
    }

    // The upstream store may call this while BaseLayout is starting monitoring;
    // share the in-flight request instead of fetching twice.
    initializing ??= loadNodes().finally(() => {
      initializing = undefined
    })

    return initializing
  }

  async function loadNodes() {
    try {
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
          auth_method: node.auth_method,
          has_credential: node.has_credential,
          credential_status: node.credential_status,
          connection_error: node.connection_error,
          connection_error_code: node.connection_error_code,
          connection_error_at: node.connection_error_at,
          enabled: true,
        }
      })

      nodes.value = nodeMap
      writeCachedNodes(nodes.value)
    }
    catch (error) {
      // The WebSocket snapshot still fills the map once connected.
      console.error('Failed to initialize node data:', error)
    }
    finally {
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

    // Trust the socket, not the flag: VueUse creates the WebSocket synchronously
    // in open(), so a stale isConnecting would only block recovery.
    if (readyState === WebSocket.CONNECTING) {
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

  // Reconnects a socket that died under a live session (sleep, backend restart,
  // a proxy dropping an idle tunnel) and rebuilds one that went silent.
  const supervisor = useConnectionSupervisor({
    socket,
    connect: connectWebSocket,
    staleAfterMs: STALE_MESSAGE_MS,
    intervalMs: WATCHDOG_INTERVAL_MS,
  })

  // Start monitoring (initialize + WebSocket). Safe to call again after
  // stopMonitoring(): every call connects, not just the first one.
  async function startMonitoring() {
    isMonitoring = true

    // Load the API snapshot first so it cannot overwrite fresher socket data.
    await initialize()

    // Logged out while the request was in flight: stay closed.
    if (!isMonitoring) {
      return
    }

    connectWebSocket()
    supervisor.start()
  }

  // Stop monitoring and cleanup. close() runs unconditionally: between
  // autoReconnect retries the socket is already CLOSED, and skipping close()
  // would let the queued retry reopen it after logout.
  function stopMonitoring() {
    isMonitoring = false
    supervisor.stop()
    socket.close()

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
