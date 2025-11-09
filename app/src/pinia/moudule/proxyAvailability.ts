import type { ProxyTarget } from '@/api/site'
import type { UpstreamAvailabilityResponse, UpstreamStatus } from '@/api/upstream'
import upstream from '@/api/upstream'
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

export const useProxyAvailabilityStore = defineStore('proxyAvailability', () => {
  const availabilityResults = ref<Record<string, ProxyAvailabilityResult>>({})
  const upstreamStatusMap = ref<UpstreamStatusMap>({})
  const websocket = shallowRef<WebSocket>()
  const nodeAnalyticsWebsocket = shallowRef<WebSocket>()
  const isConnected = ref(false)
  const isNodeAnalyticsConnected = ref(false)
  const isInitialized = ref(false)
  const lastUpdateTime = ref<string>('')
  const targetCount = ref(0)

  const nodeStore = useNodeAvailabilityStore()

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
  async function connectWebSocket() {
    if (websocket.value && isConnected.value) {
      return
    }

    // Close existing connection if any
    if (websocket.value) {
      websocket.value.close()
    }

    try {
      // Create new WebSocket connection
      const { useWebSocket } = await import('@/lib/websocket')
      const { ws } = useWebSocket(upstream.availabilityWebSocketUrl)
      websocket.value = ws.value!

      ws.value!.onopen = () => {
        isConnected.value = true
      }

      ws.value!.onmessage = (e: MessageEvent) => {
        try {
          const results = JSON.parse(e.data) as Record<string, ProxyAvailabilityResult>
          // Update availability results with latest data
          availabilityResults.value = { ...results }
          lastUpdateTime.value = new Date().toISOString()
        }
        catch (error) {
          console.error('Failed to parse WebSocket message:', error)
        }
      }

      ws.value!.onclose = () => {
        isConnected.value = false
      }

      ws.value!.onerror = error => {
        console.error('Proxy availability WebSocket error:', error)
        isConnected.value = false
      }
    }
    catch (error) {
      console.error('Failed to create WebSocket connection:', error)
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
    if (websocket.value) {
      websocket.value.close()
      websocket.value = undefined
      isConnected.value = false
    }
    if (nodeAnalyticsWebsocket.value) {
      nodeAnalyticsWebsocket.value.close()
      nodeAnalyticsWebsocket.value = undefined
      isNodeAnalyticsConnected.value = false
    }
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

  // Auto-cleanup WebSocket on page unload
  if (typeof window !== 'undefined') {
    window.addEventListener('beforeunload', () => {
      stopMonitoring()
    })
  }

  return {
    availabilityResults: readonly(availabilityResults),
    upstreamStatusMap: readonly(upstreamStatusMap),
    isConnected: readonly(isConnected),
    isNodeAnalyticsConnected: readonly(isNodeAnalyticsConnected),
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
