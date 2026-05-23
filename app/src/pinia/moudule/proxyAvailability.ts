import type { ProxyTarget } from '@/api/site'
import type { UpstreamAvailabilityResponse, UpstreamStatus } from '@/api/upstream'
import { useEventListener } from '@vueuse/core'
import upstream from '@/api/upstream'
import { useWebSocket } from '@/lib/websocket'
import { useNodeAvailabilityStore } from './nodeAvailability'

// Extended types for multi-node support
export interface NodeUpstreamStatus {
  online: boolean
  latency: number
}

export interface MultiNodeUpstreamStatus {
  [nodeId: number]: NodeUpstreamStatus
}

export interface UpstreamStatusMap {
  [targetKey: string]: MultiNodeUpstreamStatus
}

// Alias for consistency with existing code
export type ProxyAvailabilityResult = UpstreamStatus

// Grace period before flipping the UI indicator to "disconnected" — avoids
// flicker during VueUse's autoReconnect retries (10 × 1s) on transient drops.
const DISCONNECT_GRACE_MS = 1500

export const useProxyAvailabilityStore = defineStore('proxyAvailability', () => {
  const availabilityResults = ref<Record<string, ProxyAvailabilityResult>>({})
  const upstreamStatusMap = ref<UpstreamStatusMap>({})
  const isConnected = ref(false)
  const isInitialized = ref(false)
  const lastUpdateTime = ref<string>('')
  const targetCount = ref(0)

  const nodeStore = useNodeAvailabilityStore()

  let disconnectTimer: ReturnType<typeof setTimeout> | undefined
  const clearDisconnectTimer = () => {
    clearTimeout(disconnectTimer)
    disconnectTimer = undefined
  }

  const socket = useWebSocket<Record<string, ProxyAvailabilityResult>>(upstream.availabilityWebSocketUrl, true, {
    immediate: false,
    autoClose: false,
    onConnected() {
      clearDisconnectTimer()
      isConnected.value = true
    },
    onDisconnected() {
      clearDisconnectTimer()
      disconnectTimer = setTimeout(() => {
        isConnected.value = false
      }, DISCONNECT_GRACE_MS)
    },
    onError(_, error) {
      console.error('Proxy availability WebSocket error:', error)
    },
    onMessage(_, event) {
      try {
        availabilityResults.value = JSON.parse(event.data) as Record<string, ProxyAvailabilityResult>
        lastUpdateTime.value = new Date().toISOString()
      }
      catch (error) {
        console.error('Failed to parse WebSocket message:', error)
      }
    },
  })

  // Format socket address for target key (handles IPv6 addresses)
  function formatSocketAddress(host: string, port: string): string {
    // Check if this is an IPv6 address by looking for colons
    if (host.includes(':')) {
      // IPv6 address - check if it already has brackets
      if (!host.startsWith('[')) {
        return `[${host}]:${port}`
      }
      // Already has brackets, just append port
      return `${host}:${port}`
    }
    // IPv4 address or hostname
    return `${host}:${port}`
  }

  function getTargetKey(target: ProxyTarget): string {
    return formatSocketAddress(target.host, target.port)
  }

  // Initialize availability data from HTTP API
  async function initialize() {
    if (isInitialized.value) {
      return
    }

    try {
      const response = await upstream.getAvailability()
      const data = response as UpstreamAvailabilityResponse

      availabilityResults.value = data.results || {}
      lastUpdateTime.value = data.last_update_time || ''
      targetCount.value = data.target_count || 0

      isInitialized.value = true
    }
    catch (error) {
      console.error('Failed to initialize proxy availability:', error)
    }
  }

  // Connect to WebSocket for real-time updates
  function connectWebSocket() {
    const readyState = socket.ws.value?.readyState

    if (readyState === WebSocket.OPEN) {
      isConnected.value = true
      return
    }

    if (readyState === WebSocket.CONNECTING) {
      return
    }

    try {
      socket.open()
    }
    catch (error) {
      console.error('Failed to initiate WebSocket connection:', error)
    }
  }

  // Start monitoring (initialize + WebSocket)
  async function startMonitoring() {
    // Initialize node store first
    if (!nodeStore.isInitialized) {
      nodeStore.initialize()
    }

    await initialize()
    connectWebSocket()
  }

  // Stop monitoring and cleanup
  function stopMonitoring() {
    clearDisconnectTimer()

    if (socket.ws.value && socket.ws.value.readyState !== WebSocket.CLOSED) {
      socket.close()
    }

    isConnected.value = false
  }

  // Get availability result for a specific target
  function getAvailabilityResult(target: ProxyTarget): ProxyAvailabilityResult | undefined {
    const key = getTargetKey(target)
    return availabilityResults.value[key]
  }

  // Check if target has availability data
  function hasAvailabilityData(target: ProxyTarget): boolean {
    const key = getTargetKey(target)
    return key in availabilityResults.value
  }

  // Get all available targets
  function getAllTargets(): string[] {
    return Object.keys(availabilityResults.value)
  }

  // Update upstream status map from node data
  function updateUpstreamStatusMapFromNode(nodeId: number, upstreamData: Record<string, NodeUpstreamStatus>) {
    if (!upstreamData)
      return

    for (const [targetKey, status] of Object.entries(upstreamData)) {
      if (!upstreamStatusMap.value[targetKey]) {
        upstreamStatusMap.value[targetKey] = {}
      }

      // Update the status for this specific node
      upstreamStatusMap.value[targetKey][nodeId] = {
        online: status.online,
        latency: status.latency,
      }
    }
  }

  // Get multi-node status for a target
  function getMultiNodeStatus(target: ProxyTarget): MultiNodeUpstreamStatus | undefined {
    const key = getTargetKey(target)
    return upstreamStatusMap.value[key]
  }

  // Get aggregated status for a target (online nodes / total nodes)
  function getAggregatedStatus(target: ProxyTarget): { online: number, total: number, testType: string } {
    const multiNodeStatus = getMultiNodeStatus(target)
    if (!multiNodeStatus) {
      // Fallback to single-node status
      const singleStatus = getAvailabilityResult(target)
      if (singleStatus) {
        return {
          online: singleStatus.online ? 1 : 0,
          total: 1,
          testType: 'local',
        }
      }
      return { online: 0, total: 0, testType: 'local' }
    }

    const statuses = Object.values(multiNodeStatus)
    const onlineCount = statuses.filter(status => status.online).length

    return {
      online: onlineCount,
      total: statuses.length,
      testType: 'multi-node',
    }
  }

  // Auto-cleanup WebSocket on page unload. VueUse removes the listener with
  // the store scope (pinia setup stores have an effectScope), so this stays
  // a single, scope-bound registration for the lifetime of the app.
  useEventListener(window, 'beforeunload', () => stopMonitoring())

  return {
    availabilityResults: readonly(availabilityResults),
    upstreamStatusMap: readonly(upstreamStatusMap),
    isConnected: readonly(isConnected),
    isInitialized: readonly(isInitialized),
    lastUpdateTime: readonly(lastUpdateTime),
    targetCount: readonly(targetCount),
    initialize,
    startMonitoring,
    stopMonitoring,
    connectWebSocket,
    getAvailabilityResult,
    hasAvailabilityData,
    getAllTargets,
    getTargetKey,
    updateUpstreamStatusMapFromNode,
    getMultiNodeStatus,
    getAggregatedStatus,
  }
})
