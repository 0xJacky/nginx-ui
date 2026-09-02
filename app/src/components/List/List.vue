<script setup lang="ts" generic="T = any">
/**
 * Minimal replacement for ant-design-vue's List component.
 * antdv-next dropped List entirely, so we ship only the subset this project uses:
 * data-source + renderItem slot, plain default slot, header slot and the bordered variant.
 */
defineProps<{
  dataSource?: T[]
  itemLayout?: 'horizontal' | 'vertical'
  bordered?: boolean
}>()

defineSlots<{
  default?: () => unknown
  header?: () => unknown
  renderItem?: (props: { item: T, index: number }) => unknown
}>()
</script>

<template>
  <div class="nui-list" :class="{ 'nui-list-bordered': bordered }">
    <div v-if="$slots.header" class="nui-list-header">
      <slot name="header" />
    </div>
    <div class="nui-list-items">
      <template v-if="dataSource && $slots.renderItem">
        <template v-for="(item, index) in dataSource" :key="index">
          <slot name="renderItem" :item="item" :index="index" />
        </template>
      </template>
      <slot v-else />
    </div>
  </div>
</template>

<style scoped lang="less">
.nui-list {
  position: relative;
}

.nui-list-header {
  padding: 12px 0;
  border-bottom: 1px solid rgba(5, 5, 5, 0.06);
}

.nui-list-bordered {
  border: 1px solid rgba(5, 5, 5, 0.06);
  border-radius: 8px;

  .nui-list-header {
    padding: 12px 24px;
  }

  :deep(.nui-list-item) {
    padding: 12px 24px;
  }
}

.dark {
  .nui-list-header {
    border-bottom-color: rgba(253, 253, 253, 0.12);
  }

  .nui-list-bordered {
    border-color: rgba(253, 253, 253, 0.12);
  }
}
</style>
