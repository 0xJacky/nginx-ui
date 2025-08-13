<script setup lang="ts">
import type { UpdateOrderRequest } from '@/api/curd'
import { StdCurd } from '@uozi-admin/curd'
import namespace from '@/api/namespace'
import NodeSelector from '@/components/NodeSelector'
import columns from './columns'

const table = useTemplateRef('table')

async function handleDragEnd(data: UpdateOrderRequest) {
  await namespace.updateOrder(data)
  table.value?.refresh()
}
</script>

<template>
  <StdCurd
    ref="table"
    :title="$gettext('Namespaces')"
    :api="namespace"
    :columns="columns"
    :scroll-x="600"
    disable-export
    row-draggable
    :row-draggable-options="{
      onEnd: handleDragEnd,
    }"
  >
    <template #afterForm="{ record }">
      <div class="mb-2">
        {{ $gettext('Sync Nodes') }}
      </div>
      <NodeSelector
        v-model:target="record.sync_node_ids"
        hidden-local
      />
    </template>
  </StdCurd>
</template>

<style scoped lang="less">
</style>
