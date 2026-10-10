<script setup lang="ts">
import NodeSelector from '@/components/NodeSelector'

const syncNodeIds = defineModel<number[] | undefined>('syncNodeIds')

const picking = ref(false)
const hasNodes = computed(() => (syncNodeIds.value?.length ?? 0) > 0)
</script>

<template>
  <ACard size="small" :title="$gettext('Sync to other nodes')">
    <template v-if="!hasNodes && !picking" #extra>
      <AButton type="link" size="small" class="px-0" @click="picking = true">
        {{ $gettext('Select nodes') }}
      </AButton>
    </template>
    <p v-if="!hasNodes && !picking" class="sync-empty">
      {{ $gettext('Only kept on this node.') }}
    </p>
    <NodeSelector
      v-else
      v-model:target="syncNodeIds"
      hidden-local
    />
  </ACard>
</template>

<style scoped lang="less">
.sync-empty {
  margin: 0;
  color: var(--ant-color-text-secondary);
}
</style>
