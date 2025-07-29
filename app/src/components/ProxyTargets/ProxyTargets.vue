<script setup lang="ts">
import type { ProxyTarget } from '@/api/site'
import { useNodeAvailabilityStore } from '@/pinia/moudule/nodeAvailability'
import { useNodeGroupStore } from '@/pinia/moudule/nodeGroupStore'
import { useProxyAvailabilityStore } from '@/pinia/moudule/proxyAvailability'

interface Props {
  targets: ProxyTarget[]
  envGroupId?: number
}

const props = defineProps<Props>()

const proxyStore = useProxyAvailabilityStore()
const nodeStore = useNodeAvailabilityStore()
const nodeGroupStore = useNodeGroupStore()

// Initialize the stores to start monitoring
onMounted(() => {
  proxyStore.startMonitoring()
  nodeGroupStore.initialize()
})

onUnmounted(() => {
  proxyStore.stopMonitoring()
})

// Check if should show multi-node display based on group configuration
const shouldShowMultiNodeDisplay = computed(() => {
  if (!props.envGroupId) {
    return false
  }

  const group = nodeGroupStore.getGroupById(props.envGroupId)
  const testType = group?.upstream_test_type || 'local'
  return testType === 'remote' || testType === 'mirror'
})

// eslint-disable-next-line sonarjs/cognitive-complexity
function getTargetColor(target: ProxyTarget): string {
  // Check if we should show multi-node display based on group configuration
  if (shouldShowMultiNodeDisplay.value) {
    const multiNodeStatus = proxyStore.getMultiNodeStatus(target)
    const group = nodeGroupStore.getGroupById(props.envGroupId!)
    const testType = group?.upstream_test_type || 'local'

    // Calculate total nodes based on test type
    const totalNodes = testType === 'remote'
      ? (group?.sync_node_ids?.length || 0) // remote: only sync nodes
      : (group?.sync_node_ids?.length || 0) + 1 // mirror: sync nodes + main node

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

    if (onlineCount === totalNodes)
      return 'green'
    if (onlineCount === 0)
      return 'red'
    return 'orange' // Partial online
  }

  // Fallback to single-node display
  const result = proxyStore.getAvailabilityResult(target)
  if (!result)
    return 'default'
  return result.online ? 'green' : 'red'
}

function getTargetText(target: ProxyTarget): string {
  // Check if we should show multi-node display based on group configuration
  if (shouldShowMultiNodeDisplay.value) {
    const multiNodeStatus = proxyStore.getMultiNodeStatus(target)
    const group = nodeGroupStore.getGroupById(props.envGroupId!)
    const testType = group?.upstream_test_type || 'local'

    // Calculate total nodes based on test type
    const totalNodes = testType === 'remote'
      ? (group?.sync_node_ids?.length || 0) // remote: only sync nodes
      : (group?.sync_node_ids?.length || 0) + 1 // mirror: sync nodes + main node

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

    return `${target.host}:${target.port} (${onlineCount}/${totalNodes})`
  }

  // Fallback to single-node display
  const result = proxyStore.getAvailabilityResult(target)
  if (!result)
    return `${target.host}:${target.port}`

  if (result.online) {
    return `${target.host}:${target.port} (${result.latency.toFixed(2)}ms)`
  }
  else {
    return `${target.host}:${target.port}`
  }
}

function getTargetTitle(target: ProxyTarget): string {
  return `${$gettext('Type')}: ${target.type === 'upstream' ? $gettext('Upstream') : $gettext('Proxy Pass')}`
}

const showDetailModal = ref(false)
const selectedTarget = ref<ProxyTarget | null>(null)

function getNodeName(nodeId: string): string {
  const node = nodeStore.nodes[Number.parseInt(nodeId)]
  return node?.name || `Node ${nodeId}`
}

// Get all node statuses including main node for modal display
function getAllNodeStatuses(target: ProxyTarget) {
  const group = nodeGroupStore.getGroupById(props.envGroupId!)
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

function handleTargetClick(target: ProxyTarget) {
  if (shouldShowMultiNodeDisplay.value) {
    selectedTarget.value = target
    showDetailModal.value = true
  }
}
</script>

<template>
  <div v-if="targets.length > 0" class="proxy-targets">
    <ATag
      v-for="target in targets" :key="proxyStore.getTargetKey(target)" :color="getTargetColor(target)"
      class="proxy-target-tag cursor-pointer" :class="{ clickable: shouldShowMultiNodeDisplay }" :bordered="false"
      @click="handleTargetClick(target)"
    >
      <template #icon>
        <ATooltip :title="getTargetTitle(target)" placement="bottom" class="cursor-pointer">
          <span v-if="target.type === 'upstream'" class="target-type-icon">U</span>
          <span v-else class="target-type-icon">P</span>
        </ATooltip>
      </template>
      {{ getTargetText(target) }}
    </ATag>

    <!-- Upstream Detail Modal -->
    <AModal
      v-model:open="showDetailModal"
      :title="selectedTarget ? `${selectedTarget.host}:${selectedTarget.port} - ${$gettext('Node Status')}` : ''"
      :footer="null" width="600px"
    >
      <div v-if="selectedTarget" class="upstream-detail">
        <div class="node-status-list">
          <div v-for="nodeInfo in getAllNodeStatuses(selectedTarget)" :key="nodeInfo.nodeId" class="node-status-item">
            <div class="node-info">
              <span class="node-name">{{ nodeInfo.name }}</span>
              <ATag :color="nodeInfo.status.online ? 'green' : 'red'" class="status-tag">
                {{ nodeInfo.status.online ? $gettext('Online') : $gettext('Offline') }}
              </ATag>
              <ATag v-if="nodeInfo.isMainNode" color="blue" class="main-node-tag">
                {{ $gettext('Main') }}
              </ATag>
            </div>
            <div class="node-latency">
              <span v-if="nodeInfo.status.online">{{ nodeInfo.status.latency.toFixed(2) }}ms</span>
              <span v-else class="text-gray-400">{{ $gettext('N/A') }}</span>
            </div>
          </div>
        </div>
      </div>
    </AModal>
  </div>
</template>

<style scoped lang="less">
  .proxy-targets {
    display: flex;
    flex-wrap: wrap;
    gap: 4px;
    max-width: 100%;
    overflow: hidden;
  }

  .proxy-target-tag {
    margin-right: 4px;
    margin-bottom: 4px;
    font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
    font-size: 12px;
    max-width: 100%;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;

    &.clickable {
      cursor: pointer;
      transition: all 0.2s ease;

      &:hover {
        box-shadow: 0 0 8px 0 rgba(0, 0, 0, 0.07);
      }
    }

    .target-type-icon {
      display: inline-block;
      width: 12px;
      height: 12px;
      line-height: 12px;
      text-align: center;
      background: rgba(255, 255, 255, 0.2);
      border-radius: 2px;
      margin-right: 4px;
      font-weight: bold;
      font-size: 10px;
      flex-shrink: 0;
    }
  }

  .upstream-detail {
    .node-status-list {
      display: flex;
      flex-direction: column;
      gap: 12px;
    }

    .node-status-item {
      display: flex;
      justify-content: space-between;
      align-items: center;
      padding: 12px;
      border: 1px solid #e8e9ea;
      border-radius: 8px;
      background: #fafafa;

      .node-info {
        display: flex;
        align-items: center;
        gap: 8px;

        .node-name {
          font-weight: 500;
          color: #333;
        }

        .status-tag {
          font-size: 11px;
          margin: 0;
        }

        .main-node-tag {
          font-size: 10px;
          margin: 0;
        }
      }

      .node-latency {
        font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
        font-size: 12px;
        font-weight: 500;
        color: #666;
      }

    }
  }
</style>
