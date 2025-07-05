import type ReconnectingWebSocket from 'reconnecting-websocket'
import type { ProxyTarget } from '@/api/site'
import { defineStore } from 'pinia'
import upstream, { type UpstreamStatus, type UpstreamAvailabilityResponse } from '@/api/upstream'

// Alias for consistency with existing code
export type ProxyAvailabilityResult = UpstreamStatus

export const useProxyAvailabilityStore = defineStore('proxyAvailability', () => {
  const availabilityResults = ref<Record<string, ProxyAvailabilityResult>>({})
  const websocket = shallowRef<ReconnectingWebSocket | WebSocket>()
  const isConnected = ref(false)
  const isInitialized = ref(false)
  const lastUpdateTime = ref<string>('')
  const targetCount = ref(0)

  function getTargetKey(target: ProxyTarget): string {
    return `${target.host}:${target.port}`
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
      
      console.log(`Initialized proxy availability with ${targetCount.value} targets`)
    } catch (error) {
      console.error('Failed to initialize proxy availability:', error)
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
       const ws = upstream.availabilityWebSocket()
       websocket.value = ws

       ws.onopen = () => {
         isConnected.value = true
         console.log('Proxy availability WebSocket connected')
       }

       ws.onmessage = (e: MessageEvent) => {
         try {
           const results = JSON.parse(e.data) as Record<string, ProxyAvailabilityResult>
           // Update availability results with latest data
           availabilityResults.value = { ...results }
           lastUpdateTime.value = new Date().toISOString()
         } catch (error) {
           console.error('Failed to parse WebSocket message:', error)
         }
       }

       ws.onclose = () => {
         isConnected.value = false
         console.log('Proxy availability WebSocket disconnected')
       }

       ws.onerror = error => {
         console.error('Proxy availability WebSocket error:', error)
         isConnected.value = false
       }
     } catch (error) {
       console.error('Failed to create WebSocket connection:', error)
     }
  }

  // Start monitoring (initialize + WebSocket)
  async function startMonitoring() {
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

  // Auto-cleanup WebSocket on page unload
  if (typeof window !== 'undefined') {
    window.addEventListener('beforeunload', () => {
      stopMonitoring()
    })
  }

  return {
    availabilityResults: readonly(availabilityResults),
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
  }
})
