<script setup lang="ts">
import env_group, { PostSyncAction } from '@/api/env_group'
import NodeSelector from '@/components/NodeSelector/NodeSelector.vue'
import { StdCurd } from '@/components/StdDesign/StdDataDisplay'
import columns from '@/views/environments/group/columns'
</script>

<template>
  <StdCurd
    :title="$gettext('Node Groups')"
    :api="env_group"
    :columns="columns"
    :scroll-x="600"
    sortable
  >
    <template #edit="{ data }">
      <div class="mb-2">
        {{ $gettext('Sync Nodes') }}
      </div>
      <NodeSelector
        v-model:target="data.sync_node_ids"
        hidden-local
      />

      <AForm class="mt-4" layout="vertical">
        <AFormItem :label="$gettext('Post-sync Action')">
          <ASelect
            v-model:value="data.post_sync_action"
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
