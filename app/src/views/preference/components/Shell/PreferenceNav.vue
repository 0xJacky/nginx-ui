<script setup lang="ts">
import type { PreferenceGroup, PreferenceSection, PreferenceSectionKey } from '../../sections'

const props = defineProps<{
  groups: PreferenceGroup[]
  sections: PreferenceSection[]
  dirtySections: Set<PreferenceSectionKey>
}>()

const activeKey = defineModel<PreferenceSectionKey>('activeKey', { required: true })

const groupedSections = computed(() => props.groups
  .map(group => ({
    ...group,
    sections: props.sections.filter(section => section.group === group.key),
  }))
  .filter(group => group.sections.length > 0))
</script>

<template>
  <nav class="preference-nav">
    <template
      v-for="group in groupedSections"
      :key="group.key"
    >
      <div class="preference-nav-group">
        {{ group.label }}
      </div>
      <button
        v-for="section in group.sections"
        :key="section.key"
        type="button"
        class="preference-nav-item"
        :class="{ 'is-active': section.key === activeKey }"
        :aria-current="section.key === activeKey ? 'page' : undefined"
        @click="activeKey = section.key"
      >
        <span class="preference-nav-label">{{ section.label }}</span>
        <span
          v-if="dirtySections.has(section.key)"
          class="preference-nav-dot"
          :title="$gettext('Unsaved')"
        />
      </button>
    </template>
  </nav>
</template>

<style lang="less" scoped>
.preference-nav {
  display: flex;
  flex-direction: column;
  padding: 16px 12px;
  border-right: 1px solid var(--ant-color-border-secondary);
}

.preference-nav-group {
  padding: 8px 12px 4px;
  font-size: 12px;
  color: var(--ant-color-text-tertiary);

  &:not(:first-child) {
    margin-top: 8px;
  }
}

.preference-nav-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  width: 100%;
  padding: 7px 12px;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: var(--ant-color-text);
  font: inherit;
  text-align: left;
  cursor: pointer;
  transition: background-color 0.2s, color 0.2s;

  &:hover {
    background: var(--ant-color-fill-tertiary);
  }

  &.is-active {
    background: var(--ant-color-primary-bg);
    color: var(--ant-color-primary);
    font-weight: 500;
  }
}

.preference-nav-label {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.preference-nav-dot {
  flex: none;
  width: 6px;
  height: 6px;
  border-radius: 3px;
  background: var(--ant-color-warning);
}
</style>
