<script setup lang="ts">
defineProps<{
  title?: string
  description?: string
}>()
</script>

<template>
  <section class="setting-panel">
    <div
      v-if="title || $slots.title || $slots.extra"
      class="setting-panel-head"
    >
      <h3 class="setting-panel-title">
        <slot name="title">
          {{ title }}
        </slot>
      </h3>
      <div v-if="$slots.extra" class="setting-panel-extra">
        <slot name="extra" />
      </div>
    </div>
    <p
      v-if="description || $slots.description"
      class="setting-panel-desc"
    >
      <slot name="description">
        {{ description }}
      </slot>
    </p>
    <slot />
  </section>
</template>

<style lang="less" scoped>
.setting-panel {
  padding: 4px 16px;
  border: 1px solid var(--ant-color-border-secondary);
  border-radius: 12px;
  background: var(--ant-color-bg-container);

  & + & {
    margin-top: 16px;
  }
}

.setting-panel-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin: 12px 0 4px;
}

.setting-panel-title {
  margin: 0;
  font-size: 14px;
  font-weight: 600;
  line-height: 22px;
}

.setting-panel-extra {
  flex: none;
}

.setting-panel-desc {
  margin: 0 0 8px;
  font-size: 13px;
  line-height: 1.5;
  color: var(--ant-color-text-secondary);
}

// The first row after the head or description has no divider.
.setting-panel-head + :deep(.setting-row),
.setting-panel-desc + :deep(.setting-row),
:deep(.setting-row:first-child) {
  border-top: none;
}

@media (max-width: 600px) {
  .setting-panel {
    padding: 4px 12px;
  }
}
</style>
