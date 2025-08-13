<!-- Displays proxy targets as tags with upstream status monitoring -->
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
      class="proxy-target-tag" :class="{ 'clickable': shouldShowMultiNodeDisplay, 'cursor-pointer': shouldShowMultiNodeDisplay }" :bordered="false"
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

    <!-- Detail Modal -->
    <UpstreamDetailModal
      v-model:open="showDetailModal"
      :target="selectedTarget"
      :namespace-id="namespaceId"
    />
  </div>
</template>

<style scoped lang="less">
  .proxy-targets {
    display: flex;
    flex-wrap: wrap;
    gap: 4px;
    max-width: 100%;
    overflow: hidden;
    padding: 6px;
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
        box-shadow: 0 0 6px 0 rgba(0, 0, 0, 0.07);

        .dark & {
          box-shadow: 0 0 6px 0 rgba(255, 255, 255, 0.1);
        }
      }
    }

    .target-type-icon {
      display: inline-block;
      width: 12px;
      height: 12px;
      line-height: 12px;
      text-align: center;
      border-radius: 2px;
      font-weight: bold;
      font-size: 10px;
      flex-shrink: 0;
    }
  }
</style>
