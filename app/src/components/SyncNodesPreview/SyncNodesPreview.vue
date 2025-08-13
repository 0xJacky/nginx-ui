<script setup lang="ts">
import type { Namespace } from '@/api/namespace'
import namespace from '@/api/namespace'
import NodeCard from '@/components/NodeCard'

const props = defineProps<{
  namespaceId?: number | null
  syncNodeIds?: number[]
}>()

// Get namespace info
const namespaceInfo = ref<Namespace | null>(null)

watch(() => props.namespaceId, async newNamespaceId => {
  if (!newNamespaceId) {
    namespaceInfo.value = null
    return
  }

  try {
    const response = await namespace.getItem(newNamespaceId)
    namespaceInfo.value = response
  }
  catch (error) {
    console.error('Failed to fetch namespace:', error)
    namespaceInfo.value = null
  }
}, { immediate: true })

// Merge nodes from namespace and manually selected nodes
const allSyncNodeIds = computed(() => {
  const namespaceNodes = namespaceInfo.value?.sync_node_ids || []
  const manualNodes = props.syncNodeIds || []

  // Merge and deduplicate
  const allNodes = [...new Set([...namespaceNodes, ...manualNodes])]
  return allNodes
})
</script>

<template>
  <div v-if="allSyncNodeIds.length > 0" class="my-4 p-4 bg-blue-50 dark:bg-blue-900/20 rounded-lg border border-blue-200 dark:border-blue-800">
    <div class="mb-3">
      <strong class="text-blue-800 dark:text-blue-300">
        {{ $gettext('Sync Preview') }}
      </strong>
    </div>

    <div class="grid grid-cols-1 sm:grid-cols-1 md:grid-cols-2 lg:grid-cols-2 xl:grid-cols-2 2xl:grid-cols-3 3xl:grid-cols-4 gap-2">
      <NodeCard
        v-for="nodeId in allSyncNodeIds"
        :key="nodeId"
        :node-id="nodeId"
        size="sm"
      />
    </div>

    <div v-if="namespaceInfo" class="mt-2 text-xs text-gray-500 dark:text-gray-400">
      {{ $gettext('* Includes nodes from group %{groupName} and manually selected nodes', { groupName: namespaceInfo.name }) }}
    </div>
  </div>
</template>

<style scoped>
</style>
