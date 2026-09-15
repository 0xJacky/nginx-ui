<script setup lang="ts">
import type { EntryItem } from './types'
import { ArrowRightOutlined } from '@antdv-next/icons'

const props = defineProps<{
  entries: EntryItem[]
}>()

function entryKey(item: EntryItem) {
  return item.key ?? (typeof item.path === 'string' ? item.path : item.title)
}

// Reserve the icon column for every card as soon as one entry carries an icon,
// so titles stay on a single vertical line instead of stepping in and out.
const hasIcons = computed(() => props.entries.some(entry => entry.icon))
</script>

<template>
  <div class="entry-grid">
    <RouterLink
      v-for="item in props.entries"
      :key="entryKey(item)"
      :to="item.path"
      class="entry-card"
    >
      <span
        v-if="hasIcons"
        class="entry-icon"
        aria-hidden="true"
      >
        <component :is="item.icon" v-if="item.icon" />
      </span>

      <span class="entry-body">
        <span class="entry-title">{{ item.title }}</span>
        <span v-if="item.description" class="entry-description">{{ item.description }}</span>
      </span>

      <ArrowRightOutlined class="entry-arrow" aria-hidden="true" />
    </RouterLink>
  </div>
</template>

<style lang="less" scoped>
.entry-grid {
  display: grid;
  gap: 16px;
  grid-template-columns: minmax(0, 1fr);

  @media (min-width: 640px) {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  @media (min-width: 1200px) {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }

  @media (min-width: 1680px) {
    grid-template-columns: repeat(4, minmax(0, 1fr));
  }
}

.entry-card {
  display: flex;
  align-items: flex-start;
  gap: 14px;
  padding: 18px;
  border: 1px solid var(--ant-color-border-secondary);
  border-radius: var(--ant-border-radius-lg);
  background: var(--ant-color-bg-container);
  color: var(--ant-color-text);
  transition: border-color 0.2s ease;

  &:hover {
    border-color: var(--ant-color-primary-border);
  }

  &:focus-visible {
    outline: 2px solid var(--ant-color-primary);
    outline-offset: 2px;
  }
}

.entry-icon {
  flex: none;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  border-radius: 10px;
  background: var(--ant-color-fill-tertiary);
  color: var(--ant-color-text-secondary);
  font-size: 18px;
  transition: background-color 0.2s ease, color 0.2s ease;
}

.entry-card:hover .entry-icon,
.entry-card:focus-visible .entry-icon {
  background: var(--ant-color-primary-bg);
  color: var(--ant-color-primary);
}

.entry-body {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.entry-title {
  font-size: 15px;
  font-weight: 600;
  line-height: 1.5;
  color: var(--ant-color-text);
}

.entry-description {
  font-size: 13px;
  line-height: 1.6;
  color: var(--ant-color-text-secondary);
}

.entry-arrow {
  flex: none;
  margin-top: 6px;
  font-size: 12px;
  color: var(--ant-color-text-quaternary);
  opacity: 0;
  transition: opacity 0.2s ease, color 0.2s ease;
}

.entry-card:hover .entry-arrow,
.entry-card:focus-visible .entry-arrow {
  color: var(--ant-color-primary);
  opacity: 1;
}

@media (prefers-reduced-motion: reduce) {
  .entry-card,
  .entry-icon,
  .entry-arrow {
    transition: none;
  }
}
</style>
