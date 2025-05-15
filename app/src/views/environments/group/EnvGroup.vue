<script setup lang="ts">
import type { UpdateOrderRequest } from '@/api/curd'
import { StdCurd } from '@uozi-admin/curd'
import env_group, { PostSyncAction } from '@/api/env_group'
import NodeSelector from '@/components/NodeSelector'
import columns from '@/views/environments/group/columns'

const table = useTemplateRef('table')

async function handleDragEnd(data: UpdateOrderRequest) {
  await env_group.updateOrder(data)
  table.value?.refresh()
}
</script>

<template>
  <StdCurd
    ref="table"
    :title="$gettext('Node Groups')"
    :api="env_group"
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

      <AForm class="mt-4" layout="vertical">
        <AFormItem :label="$gettext('Post-sync Action')">
          <ASelect
            v-model:value="record.post_sync_action"
            :placeholder="$gettext('Select an action after sync')"
            :default-value="PostSyncAction.ReloadNginx"
            class="w-full"
          >
            <ASelectOption :value="PostSyncAction.None">
              {{ $gettext('No Action') }}
            </ASelectOption>
            <ASelectOption :value="PostSyncAction.ReloadNginx">
              {{ $gettext('Reload Nginx') }}
            </ASelectOption>
          </ASelect>
        </AFormItem>
      </AForm>
    </template>
  </StdCurd>
</template>

<style scoped lang="less">
</style>
