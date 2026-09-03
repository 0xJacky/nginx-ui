<script setup lang="ts">
/**
 * Minimal replacement for ant-design-vue's List.Item.
 * Supports the default slot plus the actions slot used across the app.
 */
defineSlots<{
  default?: () => unknown
  actions?: () => unknown
}>()
</script>

<template>
  <div class="nui-list-item">
    <div class="nui-list-item-main">
      <slot />
    </div>
    <div v-if="$slots.actions" class="nui-list-item-actions">
      <slot name="actions" />
    </div>
  </div>
</template>

<style scoped lang="less">
.nui-list-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 12px 0;
  border-bottom: 1px solid rgba(5, 5, 5, 0.06);

  &:last-child {
    border-bottom: none;
  }
}

.nui-list-item-main {
  flex: 1;
  min-width: 0;
}

.nui-list-item-actions {
  display: flex;
  align-items: center;
  gap: 16px;
  flex-shrink: 0;
}

.dark .nui-list-item {
  border-bottom-color: rgba(253, 253, 253, 0.12);
}
</style>
