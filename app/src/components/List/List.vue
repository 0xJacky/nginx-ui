<script setup lang="ts" generic="T = any">
/**
 * Minimal replacement for ant-design-vue's List component.
 * antdv-next dropped List entirely, so we ship only the subset this project uses:
 * data-source + renderItem slot, plain default slot, header slot and the bordered variant.
 */
import { Empty } from 'antdv-next'

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
        <div v-if="!dataSource.length" class="nui-list-empty">
          <Empty :image="Empty.PRESENTED_IMAGE_SIMPLE" />
        </div>
        <template v-for="(item, index) in dataSource" v-else :key="index">
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
  border-bottom: 1px solid var(--ant-color-split);
}

.nui-list-empty {
  padding: 16px;
}

.nui-list-bordered {
  border: 1px solid var(--ant-color-border);
  border-radius: 8px;

  .nui-list-header {
    padding: 12px 24px;
  }
}
</style>
