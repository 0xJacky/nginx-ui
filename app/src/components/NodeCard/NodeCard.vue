<script setup lang="ts">
import { useNodeAvailabilityStore } from '@/pinia/moudule/nodeAvailability'

const props = defineProps<{
  nodeId: number
  size?: 'sm' | 'md'
}>()

const nodeStore = useNodeAvailabilityStore()

// Get node info from store
const nodeInfo = computed(() => {
  const node = nodeStore.getNodeStatus(props.nodeId)
  return {
    name: node?.name || `Node ${props.nodeId}`,
    isOnline: node?.status ?? false,
  }
})

// Size-dependent classes
const sizeClasses = computed(() => {
  if (props.size === 'sm') {
    return {
      container: 'p-2',
      indicator: 'w-2 h-2 mr-2',
      nameText: 'text-sm',
      statusText: 'text-xs',
    }
  }
  return {
    container: 'p-3',
    indicator: 'w-3 h-3 mr-3',
    nameText: 'text-base',
    statusText: 'text-sm',
  }
})
</script>

<template>
  <div
    :class="`flex items-center bg-gray-50 dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 ${sizeClasses.container}`"
  >
    <span
      :class="`inline-block rounded-full flex-shrink-0 ${sizeClasses.indicator} ${nodeInfo.isOnline ? 'bg-green-500' : 'bg-red-500'}`"
    />
    <div class="flex-1 min-w-0">
      <div :class="`font-medium truncate text-gray-900 dark:text-gray-100 ${sizeClasses.nameText}`">
        {{ nodeInfo.name }}
      </div>
      <div :class="`text-gray-500 dark:text-gray-400 ${sizeClasses.statusText}`">
        {{ nodeInfo.isOnline ? $gettext('Online') : $gettext('Offline') }}
      </div>
    </div>
  </div>
</template>

<style scoped>
</style>
