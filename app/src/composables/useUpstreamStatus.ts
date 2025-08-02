// Composable for managing upstream status logic shared between components
import type { EnvGroup } from '@/api/env_group'
import type { ProxyTarget } from '@/api/site'
import { useNodeAvailabilityStore } from '@/pinia/moudule/nodeAvailability'
import { useNodeGroupStore } from '@/pinia/moudule/nodeGroupStore'
import { useProxyAvailabilityStore } from '@/pinia/moudule/proxyAvailability'

export function useUpstreamStatus(envGroupId?: Ref<number | undefined>) {
  const proxyStore = useProxyAvailabilityStore()
  const nodeStore = useNodeAvailabilityStore()
  const nodeGroupStore = useNodeGroupStore()

  // Initialize stores on mount
  onMounted(() => {
    proxyStore.startMonitoring()
    nodeGroupStore.initialize()
  })

  onUnmounted(() => {
    proxyStore.stopMonitoring()
  })

  // Check if should show multi-node display based on group configuration
  const shouldShowMultiNodeDisplay = computed(() => {
    if (!envGroupId?.value) {
      return false
    }

    const group = nodeGroupStore.getGroupById(envGroupId.value)
    const testType = group?.upstream_test_type || 'local'
    return testType === 'remote' || testType === 'mirror'
  })

  // Get target color based on online status
  function getTargetColor(target: ProxyTarget): string {
    if (!shouldShowMultiNodeDisplay.value) {
      // Fallback to single-node display
      const result = proxyStore.getAvailabilityResult(target)
      if (!result)
        return 'default'
      return result.online ? 'green' : 'red'
    }

    return getMultiNodeColor(target)
  }

  // Helper function to get color for multi-node display
  function getMultiNodeColor(target: ProxyTarget): string {
    const group = nodeGroupStore.getGroupById(envGroupId!.value!)
    const testType = group?.upstream_test_type || 'local'
    const totalNodes = calculateTotalNodes(group, testType)
    const onlineCount = calculateOnlineCount(target, group, testType)

    if (onlineCount === totalNodes)
      return 'green'
    if (onlineCount === 0)
      return 'red'
    return 'orange' // Partial online
  }

  // Calculate total nodes based on test type
  function calculateTotalNodes(group: EnvGroup | undefined, testType: string): number {
    return testType === 'remote'
      ? (group?.sync_node_ids?.length || 0) // remote: only sync nodes
      : (group?.sync_node_ids?.length || 0) + 1 // mirror: sync nodes + main node
  }

  // Calculate online nodes count
  function calculateOnlineCount(target: ProxyTarget, group: EnvGroup | undefined, testType: string): number {
    const multiNodeStatus = proxyStore.getMultiNodeStatus(target)
    let onlineCount = 0

    if (multiNodeStatus) {
      // Count online nodes from multi-node data
      onlineCount = Object.values(multiNodeStatus).filter(status => status.online).length
    }

    // For mirror mode, also include main node status
    if (testType === 'mirror') {
      const mainNodeStatus = proxyStore.getAvailabilityResult(target)
      if (mainNodeStatus && mainNodeStatus.online) {
        onlineCount++
      }
    }

    return onlineCount
  }

  // Format socket address for display (handles IPv6 addresses)
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

  // Get target display text
  function getTargetText(target: ProxyTarget): string {
    const socketAddress = formatSocketAddress(target.host, target.port)

    if (!shouldShowMultiNodeDisplay.value) {
      // Fallback to single-node display
      const result = proxyStore.getAvailabilityResult(target)
      if (!result)
        return socketAddress

      if (result.online) {
        return `${socketAddress} (${result.latency.toFixed(2)}ms)`
      }
      else {
        return socketAddress
      }
    }

    const group = nodeGroupStore.getGroupById(envGroupId!.value!)
    const testType = group?.upstream_test_type || 'local'
    const totalNodes = calculateTotalNodes(group, testType)
    const onlineCount = calculateOnlineCount(target, group, testType)

    return `${socketAddress} (${onlineCount}/${totalNodes})`
  }

  // Get target tooltip title
  function getTargetTitle(target: ProxyTarget): string {
    return `${$gettext('Type')}: ${target.type === 'upstream' ? $gettext('Upstream') : $gettext('Proxy Pass')}`
  }

  // Get node name by ID
  function getNodeName(nodeId: string): string {
    const node = nodeStore.nodes[Number.parseInt(nodeId)]
    return node?.name || `Node ${nodeId}`
  }

  // Get all node statuses for modal display
  function getAllNodeStatuses(target: ProxyTarget) {
    if (!envGroupId?.value)
      return []

    const group = nodeGroupStore.getGroupById(envGroupId.value)
    const testType = group?.upstream_test_type || 'local'
    const allStatuses: Array<{ nodeId: string, name: string, status: { online: boolean, latency: number }, isMainNode: boolean }> = []

    // Add main node data first for local and mirror modes
    if (testType === 'local' || testType === 'mirror') {
      const mainNodeStatus = proxyStore.getAvailabilityResult(target)
      if (mainNodeStatus) {
        allStatuses.push({
          nodeId: 'main',
          name: $gettext('Main Node'),
          status: {
            online: mainNodeStatus.online,
            latency: mainNodeStatus.latency,
          },
          isMainNode: true,
        })
      }
    }

    // Add all child nodes data (both online and offline)
    if (group?.sync_node_ids) {
      const multiNodeStatus = proxyStore.getMultiNodeStatus(target)

      for (const nodeId of group.sync_node_ids) {
        const nodeIdStr = nodeId.toString()
        const nodeStatus = multiNodeStatus?.[nodeIdStr]

        allStatuses.push({
          nodeId: nodeIdStr,
          name: getNodeName(nodeIdStr),
          status: nodeStatus || {
            online: false,
            latency: 0,
          },
          isMainNode: false,
        })
      }
    }

    return allStatuses
  }

  // Get status summary for modal
  function getStatusSummary(target: ProxyTarget) {
    const allStatuses = getAllNodeStatuses(target)
    const onlineNodes = allStatuses.filter(node => node.status.online)
    const totalNodes = allStatuses.length
    const onlineCount = onlineNodes.length

    let avgLatency = 0
    if (onlineCount > 0) {
      avgLatency = onlineNodes.reduce((sum, node) => sum + node.status.latency, 0) / onlineCount
    }

    return {
      onlineCount,
      totalNodes,
      avgLatency,
    }
  }

  return {
    shouldShowMultiNodeDisplay,
    getTargetColor,
    getTargetText,
    getTargetTitle,
    getNodeName,
    getAllNodeStatuses,
    getStatusSummary,
    proxyStore,
  }
}
