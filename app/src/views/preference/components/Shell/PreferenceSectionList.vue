<script setup lang="ts">
import type { PreferenceGroup, PreferenceSection, PreferenceSectionKey } from '../../sections'
import { RightOutlined } from '@antdv-next/icons'

const props = defineProps<{
  groups: PreferenceGroup[]
  sections: PreferenceSection[]
  dirtySections: Set<PreferenceSectionKey>
}>()

const emit = defineEmits<{
  select: [key: PreferenceSectionKey]
}>()

const groupedSections = computed(() => props.groups
  .map(group => ({
    ...group,
    sections: props.sections.filter(section => section.group === group.key),
  }))
  .filter(group => group.sections.length > 0))
</script>

<template>
  <nav class="preference-section-list">
    <template
      v-for="group in groupedSections"
      :key="group.key"
    >
      <div class="preference-section-list-group">
        {{ group.label }}
      </div>
      <div class="preference-section-list-card">
        <button
          v-for="section in group.sections"
          :key="section.key"
          type="button"
          class="preference-section-list-item"
          @click="emit('select', section.key)"
        >
          <span class="preference-section-list-text">
            <span class="preference-section-list-title">{{ section.label }}</span>
            <span class="preference-section-list-desc">{{ section.description }}</span>
          </span>
          <span
            v-if="dirtySections.has(section.key)"
            class="preference-section-list-dot"
            :title="$gettext('Unsaved')"
          />
          <RightOutlined class="preference-section-list-arrow" />
        </button>
      </div>
    </template>
  </nav>
</template>

<style lang="less" scoped>
.preference-section-list {
  padding: 4px 12px 16px;
}

.preference-section-list-group {
  padding: 12px 4px 6px;
  font-size: 12px;
  color: var(--ant-color-text-tertiary);
}

.preference-section-list-card {
  overflow: hidden;
  border: 1px solid var(--ant-color-border-secondary);
  border-radius: 12px;
  background: var(--ant-color-bg-container);
}

.preference-section-list-item {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  min-height: 56px;
  padding: 10px 12px;
  border: none;
  border-top: 1px solid var(--ant-color-split);
  background: transparent;
  color: var(--ant-color-text);
  font: inherit;
  text-align: left;
  cursor: pointer;

  &:first-child {
    border-top: none;
  }

  &:active {
    background: var(--ant-color-fill-tertiary);
  }
}

.preference-section-list-text {
  display: flex;
  flex: 1 1 auto;
  flex-direction: column;
  min-width: 0;
}

.preference-section-list-title {
  font-weight: 500;
}

.preference-section-list-desc {
  overflow: hidden;
  font-size: 12px;
  line-height: 1.5;
  color: var(--ant-color-text-secondary);
  text-overflow: ellipsis;
  white-space: nowrap;
}

.preference-section-list-dot {
  flex: none;
  width: 6px;
  height: 6px;
  border-radius: 3px;
  background: var(--ant-color-warning);
}

.preference-section-list-arrow {
  flex: none;
  font-size: 12px;
  color: var(--ant-color-text-quaternary);
}
</style>
