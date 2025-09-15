<script setup lang="ts">
import type { ProxyTarget } from '@/api/site'
import UpstreamDetailModal from '@/components/UpstreamDetailModal/UpstreamDetailModal.vue'
import { useUpstreamStatus } from '@/composables/useUpstreamStatus'

interface Props {
  targets: ProxyTarget[]
  namespaceId?: number
}

const props = defineProps<Props>()

const namespaceIdRef = computed(() => props.namespaceId)
const {
  shouldShowMultiNodeDisplay,
  getTargetColor,
  getTargetText,
  getTargetTitle,
  proxyStore,
} = useUpstreamStatus(namespaceIdRef)

const showDetailModal = ref(false)
const selectedTarget = ref<ProxyTarget | null>(null)

// Handle card click to show modal
function handleCardClick(target: ProxyTarget) {
  if (shouldShowMultiNodeDisplay.value) {
    selectedTarget.value = target
    showDetailModal.value = true
  }
}

// Get card status indicator color
function getCardStatusColor(target: ProxyTarget): string {
  const color = getTargetColor(target)
  switch (color) {
    case 'green': return '#52c41a'
    case 'red': return '#ff4d4f'
    case 'orange': return '#fa8c16'
    default: return '#d9d9d9'
  }
}
</script>

<template>
  <div v-if="targets.length > 0" class="upstream-cards">
    <div class="upstream-header">
      <h3 class="upstream-title">
        Upstreams
      </h3>
      <span class="upstream-count">{{ targets.length }}</span>
    </div>
    <div class="cards-grid">
      <div
        v-for="target in targets"
        :key="proxyStore.getTargetKey(target)"
        class="upstream-card"
        :class="{ clickable: shouldShowMultiNodeDisplay }"
        @click="handleCardClick(target)"
      >
        <!-- Card content -->
        <div class="card-content">
          <div class="card-info">
            <ABadge :color="getCardStatusColor(target)" />
            <span class="card-status-text">{{ getTargetText(target) }}</span>
            <ATooltip :title="getTargetTitle(target)" placement="bottom">
              <ATag
                :color="target.type === 'upstream' ? 'blue' : 'purple'"
                size="small"
                class="type-tag"
              >
                {{ target.type === 'upstream' ? 'U' : 'P' }}
              </ATag>
            </ATooltip>
          </div>
        </div>
      </div>
    </div>

    <!-- Detail Modal -->
    <UpstreamDetailModal
      v-model:open="showDetailModal"
      :target="selectedTarget"
      :namespace-id="namespaceId"
    />
  </div>
</template>

<style scoped lang="less">
.upstream-cards {
  padding: 24px 12px 0 12px;
  margin-bottom: 16px;

  .upstream-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 12px;

    .upstream-title {
      margin: 0;
      font-size: 14px;
      font-weight: 600;
      color: #333;

      .dark & {
        color: #fff;
      }
    }

    .upstream-count {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      min-width: 18px;
      height: 18px;
      padding: 0 5px;
      background-color: #f0f0f0;
      color: #666;
      font-size: 11px;
      font-weight: 500;
      border-radius: 50%;

      .dark & {
        background-color: #434343;
        color: #ccc;
      }
    }
  }

  .cards-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
    gap: 10px;
  }

  .upstream-card {
    border: 1px solid #e8e9ea;
    border-radius: 6px;
    background: #ffffff;
    transition: all 0.2s ease;

    .dark & {
      border-color: #434343;
      background: #1f1f1f;
    }

    &.clickable {
      cursor: pointer;

      &:hover {
        box-shadow: 0 0 6px rgba(0, 0, 0, 0.08);

        .dark & {
          box-shadow: 0 0 6px rgba(255, 255, 255, 0.08);
        }
      }
    }

    .card-content {
      padding: 10px 12px;

      .card-info {
        display: flex;
        align-items: center;
        gap: 6px;

        .card-status-text {
          font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
          font-size: 11px;
          color: #666;
          line-height: 1.3;
          flex: 1;
          overflow: hidden;
          text-overflow: ellipsis;
          white-space: nowrap;

          .dark & {
            color: #999;
          }
        }

        .type-tag {
          margin: 0;
          font-size: 9px;
          font-weight: bold;
          border-radius: 3px;
          padding: 0 4px;
          line-height: 14px;
        }
      }
    }
  }
}
</style>
