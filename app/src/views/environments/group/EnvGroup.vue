<script setup lang="ts">
import env_group from '@/api/env_group'
import NodeSelector from '@/components/NodeSelector/NodeSelector.vue'
import { StdCurd } from '@/components/StdDesign/StdDataDisplay'
import columns from '@/views/environments/group/columns'
</script>

<template>
  <StdCurd
    :title="$gettext('Environment Groups')"
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
    </template>
  </StdCurd>
</template>

<style scoped lang="less">
</style>
