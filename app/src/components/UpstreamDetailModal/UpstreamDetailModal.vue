<!-- Reusable modal component for displaying upstream node status details -->
<script setup lang="ts">
import type { ProxyTarget } from '@/api/site'
import { useUpstreamStatus } from '@/composables/useUpstreamStatus'

interface Props {
  open: boolean
  target: ProxyTarget | null
  envGroupId?: number
}

const props = defineProps<Props>()
defineEmits<{
  'update:open': [value: boolean]
}>()

const envGroupIdRef = computed(() => props.envGroupId)
const { getAllNodeStatuses, getStatusSummary } = useUpstreamStatus(envGroupIdRef)
</script>

<template>
  <AModal
    :open="props.open"
    :title="props.target ? `${props.target.host}:${props.target.port} - ${$gettext('Node Status')}` : ''"
    :footer="null"
    width="600px"
    @update:open="$emit('update:open', $event)"
  >
    <div v-if="props.target" class="upstream-detail">
      <div class="summary-section">
        <div class="summary-item">
          <span class="summary-label">{{ $gettext('Online Count') }}</span>
          <span class="summary-value">{{ getStatusSummary(props.target).onlineCount }}/{{ getStatusSummary(props.target).totalNodes }}</span>
        </div>
        <div class="summary-item">
          <span class="summary-label">{{ $gettext('Average Latency') }}</span>
          <span class="summary-value">
            {{ getStatusSummary(props.target).avgLatency > 0 ? `${getStatusSummary(props.target).avgLatency.toFixed(2)}ms` : $gettext('N/A') }}
          </span>
        </div>
      </div>

      <div class="node-status-list">
        <div v-for="nodeInfo in getAllNodeStatuses(props.target)" :key="nodeInfo.nodeId" class="node-status-item">
          <div class="node-info">
            <span class="node-name">{{ nodeInfo.name }}</span>
            <ATag :color="nodeInfo.status.online ? 'green' : 'red'" class="status-tag">
              {{ nodeInfo.status.online ? $gettext('Online') : $gettext('Offline') }}
            </ATag>
            <ATag v-if="nodeInfo.isMainNode" color="blue" class="main-node-tag">
              {{ $gettext('Main Node') }}
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
</template>

<style scoped lang="less">
.upstream-detail {
  .summary-section {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 16px;
    margin-bottom: 16px;
    border-radius: 8px;
    background: #f8f9fa;

    .dark & {
      background: #2c2c2c;
    }

    .summary-item {
      display: flex;
      flex-direction: column;
      align-items: center;
      gap: 4px;

      .summary-label {
        font-size: 12px;
        color: #666;
        font-weight: 500;

        .dark & {
          color: #999;
        }
      }

      .summary-value {
        font-size: 16px;
        font-weight: 600;
        color: #333;
        font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;

        .dark & {
          color: #e6e6e6;
        }
      }
    }
  }

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

    .dark & {
      border-color: #434343;
      background: #2c2c2c;
    }

    .node-info {
      display: flex;
      align-items: center;
      gap: 8px;

      .node-name {
        font-weight: 500;
        color: #333;

        .dark & {
          color: #e6e6e6;
        }
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

      .dark & {
        color: #999;
      }
    }
  }
}
</style>
