<script setup lang="ts">
import type { PluginNodeResult } from '@/api/plugin_sync'

const props = defineProps<{
  results: PluginNodeResult[]
}>()
</script>

<template>
  <div class="sync-results">
    <div
      v-for="result in props.results"
      :key="result.node_id"
      class="sync-result-row"
    >
      <span class="font-medium">{{ result.node }}</span>
      <ATag v-if="result.success" color="green" class="m-0">
        {{ result.actions.length > 0 ? result.actions.join(', ') : $gettext('Already in sync') }}
      </ATag>
      <ATooltip v-else :title="result.error">
        <ATag color="red" class="m-0">
          {{ $gettext('Failed') }}
        </ATag>
      </ATooltip>
    </div>
  </div>
</template>

<style lang="less" scoped>
.sync-results {
  border: 1px solid var(--ant-color-border-secondary);
  border-radius: var(--ant-border-radius);
  overflow: hidden;
}

.sync-result-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 6px 12px;

  & + & {
    border-top: 1px solid var(--ant-color-border-secondary);
  }
}
</style>
