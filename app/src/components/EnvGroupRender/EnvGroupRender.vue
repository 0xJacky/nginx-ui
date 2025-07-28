<script setup lang="ts">
import type { EnvGroup } from '@/api/env_group'
import NodeCard from '@/components/NodeCard'

defineProps<{
  envGroup: EnvGroup | null
}>()

const modalVisible = ref(false)

function showModal() {
  modalVisible.value = true
}

function handleCancel() {
  modalVisible.value = false
}
</script>

<template>
  <div v-if="envGroup">
    <span
      class="cursor-pointer text-blue-500 hover:text-blue-700"
      @click="showModal"
    >
      {{ envGroup.name }}
    </span>

    <AModal
      v-model:open="modalVisible"
      :title="envGroup.name"
      :footer="null"
      width="680px"
      @cancel="handleCancel"
    >
      <div class="py-4">
        <div class="mb-4">
          <strong class="text-gray-900 dark:text-gray-100">{{ $gettext('Post-sync Action') }}:</strong>
          <span class="ml-2 text-gray-700 dark:text-gray-300">
            <template v-if="!envGroup.post_sync_action || envGroup.post_sync_action === 'none'">
              {{ $gettext('No Action') }}
            </template>
            <template v-else-if="envGroup.post_sync_action === 'reload_nginx'">
              {{ $gettext('Reload Nginx') }}
            </template>
            <template v-else>
              {{ envGroup.post_sync_action }}
            </template>
          </span>
        </div>

        <div>
          <strong class="text-gray-900 dark:text-gray-100">{{ $gettext('Sync Nodes') }}</strong>
          <div v-if="!envGroup.sync_node_ids || envGroup.sync_node_ids.length === 0" class="mt-2 text-gray-400 dark:text-gray-500">
            {{ $gettext('No nodes selected') }}
          </div>
          <div v-else class="mt-2">
            <div class="grid grid-cols-1 sm:grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-3">
              <NodeCard
                v-for="nodeId in envGroup.sync_node_ids"
                :key="nodeId"
                :node-id="nodeId"
                size="sm"
              />
            </div>
          </div>
        </div>
      </div>
    </AModal>
  </div>
  <span v-else class="text-gray-400">-</span>
</template>

<style lang="less" scoped>
</style>
