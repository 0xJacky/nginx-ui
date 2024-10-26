<script setup lang="ts">
import site_category from '@/api/site_category'
import NodeSelector from '@/components/NodeSelector/NodeSelector.vue'
import { StdCurd } from '@/components/StdDesign/StdDataDisplay'
import columns from '@/views/site/site_category/columns'
</script>

<template>
  <StdCurd
    :title="$gettext('Site Categories')"
    :api="site_category"
    :columns="columns"
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
