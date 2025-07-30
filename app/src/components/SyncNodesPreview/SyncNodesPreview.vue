<script setup lang="ts">
import type { EnvGroup } from '@/api/env_group'
import envGroup from '@/api/env_group'
import NodeCard from '@/components/NodeCard'

const props = defineProps<{
  envGroupId?: number | null
  syncNodeIds?: number[]
}>()

// Get environment group info
const envGroupInfo = ref<EnvGroup | null>(null)

watch(() => props.envGroupId, async newEnvGroupId => {
  if (!newEnvGroupId) {
    envGroupInfo.value = null
    return
  }

  try {
    const response = await envGroup.getItem(newEnvGroupId)
    envGroupInfo.value = response
  }
  catch (error) {
    console.error('Failed to fetch env group:', error)
    envGroupInfo.value = null
  }
}, { immediate: true })

// Merge nodes from env group and manually selected nodes
const allSyncNodeIds = computed(() => {
  const envGroupNodes = envGroupInfo.value?.sync_node_ids || []
  const manualNodes = props.syncNodeIds || []

  // Merge and deduplicate
  const allNodes = [...new Set([...envGroupNodes, ...manualNodes])]
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

    <div v-if="envGroupInfo" class="mt-2 text-xs text-gray-500 dark:text-gray-400">
      {{ $gettext('* Includes nodes from group %{groupName} and manually selected nodes', { groupName: envGroupInfo.name }) }}
    </div>
  </div>
</template>

<style scoped>
</style>
