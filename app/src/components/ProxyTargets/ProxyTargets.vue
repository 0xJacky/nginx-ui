<script setup lang="ts">
import type { ProxyTarget } from '@/api/site'
import { useProxyAvailabilityStore } from '@/pinia/moudule/proxyAvailability'

interface Props {
  targets: ProxyTarget[]
}

defineProps<Props>()

const proxyStore = useProxyAvailabilityStore()

function getTargetColor(target: ProxyTarget): string {
  const result = proxyStore.getAvailabilityResult(target)
  if (!result)
    return 'default'
  return result.online ? 'green' : 'red'
}

function getTargetText(target: ProxyTarget): string {
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
</script>

<template>
  <div v-if="targets.length > 0" class="proxy-targets">
    <ATag
      v-for="target in targets"
      :key="proxyStore.getTargetKey(target)"
      :color="getTargetColor(target)"
      class="proxy-target-tag"
      :bordered="false"
    >
      <template #icon>
        <ATooltip
          :title="getTargetTitle(target)"
          placement="bottom"
          class="cursor-pointer"
        >
          <span v-if="target.type === 'upstream'" class="target-type-icon">U</span>
          <span v-else class="target-type-icon">P</span>
        </ATooltip>
      </template>
      {{ getTargetText(target) }}
    </ATag>
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
</style>
