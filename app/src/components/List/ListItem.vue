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
  // Matches ant-design-vue 4's List item padding (paddingContentVertical / paddingContentHorizontalLG)
  padding: 12px 24px;
  border-bottom: 1px solid var(--ant-color-split);

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
  // With the item's 16px gap this restores ant-design-vue 4's 48px (marginXXL) action offset
  margin-inline-start: 32px;
}
</style>
