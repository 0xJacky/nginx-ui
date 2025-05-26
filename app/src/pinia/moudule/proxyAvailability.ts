import type ReconnectingWebSocket from 'reconnecting-websocket'
import type { ProxyTarget } from '@/api/site'
import { debounce } from 'lodash'
import { defineStore } from 'pinia'
import upstream from '@/api/upstream'

export interface ProxyAvailabilityResult {
  online: boolean
  latency: number
}

export const useProxyAvailabilityStore = defineStore('proxyAvailability', () => {
  const availabilityResults = ref<Record<string, ProxyAvailabilityResult>>({})
  const websocket = shallowRef<ReconnectingWebSocket | WebSocket>()
  const isConnected = ref(false)

  // Map to store targets for each component instance
  const componentTargets = ref<Map<string, string[]>>(new Map())

  // Computed property to get unique targets from all components
  const allTargets = computed(() => {
    const allTargetsList: string[] = []
    componentTargets.value.forEach(targets => {
      allTargetsList.push(...targets)
    })
    return [...new Set(allTargetsList)]
  })

  function getTargetKey(target: ProxyTarget): string {
    return `${target.host}:${target.port}`
  }

  // Debounced function to update targets on server
  const debouncedUpdateTargets = debounce(() => {
    if (websocket.value && isConnected.value) {
      websocket.value.send(JSON.stringify(allTargets.value))
    }
  }, 300)

  function ensureWebSocketConnection() {
    if (websocket.value && isConnected.value) {
      return
    }

    // Close existing connection if any
    if (websocket.value) {
      websocket.value.close()
    }

    // Create new WebSocket connection
    websocket.value = upstream.availability_test()

    websocket.value.onopen = () => {
      isConnected.value = true
      // Send current targets immediately after connection
      debouncedUpdateTargets()
    }

    websocket.value.onmessage = (e: MessageEvent) => {
      const results = JSON.parse(e.data) as Record<string, ProxyAvailabilityResult>
      // Update availability results
      Object.assign(availabilityResults.value, results)
    }

    websocket.value.onclose = () => {
      isConnected.value = false
    }

    websocket.value.onerror = error => {
      console.error('WebSocket error:', error)
      isConnected.value = false
    }
  }

  function registerComponent(targets: ProxyTarget[]): string {
    const componentId = useId()
    const targetKeys = targets.map(getTargetKey)

    componentTargets.value.set(componentId, targetKeys)

    // Ensure WebSocket connection exists
    ensureWebSocketConnection()

    // Update targets on server (debounced)
    debouncedUpdateTargets()

    return componentId
  }

  function updateComponentTargets(componentId: string, targets: ProxyTarget[]) {
    const targetKeys = targets.map(getTargetKey)
    componentTargets.value.set(componentId, targetKeys)

    // Update targets on server (debounced)
    debouncedUpdateTargets()
  }

  function unregisterComponent(componentId: string) {
    componentTargets.value.delete(componentId)

    // Update targets on server (debounced)
    debouncedUpdateTargets()

    // Close WebSocket if no components are registered
    if (componentTargets.value.size === 0) {
      // Cancel pending debounced calls
      debouncedUpdateTargets.cancel()

      if (websocket.value) {
        websocket.value.close()
        websocket.value = undefined
        isConnected.value = false
      }
    }
  }

  function getAvailabilityResult(target: ProxyTarget): ProxyAvailabilityResult | undefined {
    const key = getTargetKey(target)
    return availabilityResults.value[key]
  }

  function isTargetTesting(target: ProxyTarget): boolean {
    const key = getTargetKey(target)
    return allTargets.value.includes(key)
  }

  // Watch for changes in allTargets and update server (debounced)
  watch(allTargets, () => {
    debouncedUpdateTargets()
  })

  return {
    availabilityResults: readonly(availabilityResults),
    isConnected: readonly(isConnected),
    registerComponent,
    updateComponentTargets,
    unregisterComponent,
    getAvailabilityResult,
    isTargetTesting,
    getTargetKey,
  }
})
