<script setup lang="ts">
/**
 * Minimal replacement for ant-design-vue's List.Item.Meta.
 * Renders the avatar / title / description trio used across the app.
 */
defineProps<{
  title?: string
  description?: string
}>()

defineSlots<{
  avatar?: () => unknown
  title?: () => unknown
  description?: () => unknown
}>()
</script>

<template>
  <div class="nui-list-item-meta">
    <div v-if="$slots.avatar" class="nui-list-item-meta-avatar">
      <slot name="avatar" />
    </div>
    <div class="nui-list-item-meta-content">
      <div v-if="$slots.title || title" class="nui-list-item-meta-title">
        <slot name="title">
          {{ title }}
        </slot>
      </div>
      <div v-if="$slots.description || description" class="nui-list-item-meta-description">
        <slot name="description">
          {{ description }}
        </slot>
      </div>
    </div>
  </div>
</template>

<style scoped lang="less">
.nui-list-item-meta {
  display: flex;
  align-items: flex-start;
  gap: 16px;
  flex: 1;
  min-width: 0;
}

.nui-list-item-meta-avatar {
  flex-shrink: 0;
}

.nui-list-item-meta-content {
  flex: 1;
  min-width: 0;
}

.nui-list-item-meta-title {
  margin-bottom: 4px;
  font-size: 14px;
  line-height: 1.5715;
}

.nui-list-item-meta-description {
  font-size: 14px;
  line-height: 1.5715;
  color: rgba(0, 0, 0, 0.45);
}

.dark .nui-list-item-meta-description {
  color: rgba(255, 255, 255, 0.45);
}
</style>
