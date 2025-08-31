<script setup lang="ts">
import type { Key } from 'ant-design-vue/es/_util/type'
import { storeToRefs } from 'pinia'
import { useSettingsStore } from '@/pinia'

export interface TabOption {
  key: string
  label: string
  icon?: VNode | Component
  color?: string
  disabled?: boolean
}

interface Props {
  options: TabOption[]
  counts?: Record<string, number>
  activeKey?: string
  size?: 'small' | 'middle' | 'large'
}

interface Emits {
  (e: 'change', key: Key): void
  (e: 'update:activeKey', key: string): void
}

const props = withDefaults(defineProps<Props>(), {
  counts: () => ({}),
  activeKey: '',
  size: 'middle',
})

const emit = defineEmits<Emits>()

const settings = useSettingsStore()
const { theme } = storeToRefs(settings)

const isDarkMode = computed(() => theme.value === 'dark')

const currentActiveKey = computed({
  get: () => props.activeKey,
  set: value => emit('update:activeKey', value),
})

function handleTabChange(key: Key) {
  const keyStr = key as string
  currentActiveKey.value = keyStr
  emit('change', key)
}
</script>

<template>
  <ATabs
    :active-key="currentActiveKey"
    class="tab-filter mb-4" :class="[{ 'tab-filter-dark': isDarkMode }]"
    :size="size"
    @change="handleTabChange"
  >
    <template #rightExtra>
      <slot name="rightExtra" />
    </template>

    <ATabPane
      v-for="option in options"
      :key="option.key"
      :disabled="option.disabled"
    >
      <template #tab>
        <div
          class="tab-content flex items-center gap-1.5"
          :style="{ color: option.color || '#1890ff' }"
        >
          <span
            v-if="option.icon"
            class="tab-icon-wrapper flex items-center text-base"
          >
            <component :is="option.icon" />
          </span>
          <span class="tab-label font-medium">{{ option.label }}</span>
          <ABadge
            v-if="counts && counts[option.key] !== undefined && counts[option.key] > 0"
            :count="counts[option.key]"
            :number-style="{
              backgroundColor: option.color || '#1890ff',
              fontSize: '10px',
              height: '16px',
              lineHeight: '16px',
              minWidth: '16px',
              marginLeft: '6px',
              color: '#ffffff',
              border: 'none',
            }"
          />
        </div>
      </template>
      <slot
        :name="`content-${option.key}`"
        :option="option"
      />
    </ATabPane>
  </ATabs>
</template>

<style scoped>
/* Main Tab Filter Styling */
.tab-filter {
  --border-color: #e8e8e8;
  --primary-color: #1890ff;
  --white: #ffffff;
  --transparent: transparent;
}

/* Tab Navigation */
.tab-filter :deep(.ant-tabs-nav) {
  margin: 0;
  padding: 0;
  border-bottom: 1px solid var(--border-color);
}

.tab-filter :deep(.ant-tabs-nav::before) {
  border: none;
}

/* Tab Items */
.tab-filter :deep(.ant-tabs-tab) {
  background: var(--transparent);
  border: none;
  margin: 0;
  padding: 12px 16px;
}

/* Active Tab State */
.tab-filter :deep(.ant-tabs-tab.ant-tabs-tab-active) {
  background: transparent;
  border-bottom: 2px solid var(--primary-color);
}

.tab-filter :deep(.ant-tabs-tab.ant-tabs-tab-active) .tab-content {
  padding-bottom: 0 !important;
}

.tab-filter :deep(.ant-tabs-tab.ant-tabs-tab-active) .ant-tabs-tab-btn {
  text-shadow: unset !important;
}

/* Disabled Tab State */
.tab-filter :deep(.ant-tabs-tab.ant-tabs-tab-disabled) .tab-content {
  opacity: 0.5;
  cursor: not-allowed;
}

/* Tab Content Layout */
.tab-filter .tab-content {
  font-size: 14px;
  padding-bottom: 2px;
}

/* Size Variations */
/* Small Size */
.tab-filter:deep(.ant-tabs-small) .ant-tabs-tab {
  padding: 8px 12px;
}

.tab-filter:deep(.ant-tabs-small) .tab-content {
  font-size: 12px;
  gap: 4px;
}

.tab-filter:deep(.ant-tabs-small) .tab-icon-wrapper {
  font-size: 14px;
}

/* Large Size */
.tab-filter:deep(.ant-tabs-large) .ant-tabs-tab {
  padding: 16px 20px;
}

.tab-filter:deep(.ant-tabs-large) .tab-content {
  font-size: 16px;
  gap: 8px;
}

.tab-filter:deep(.ant-tabs-large) .tab-icon-wrapper {
  font-size: 18px;
}

/* Dark Mode Support */
.tab-filter-dark {
  --border-color: #303030;
  --white: #1f1f1f;
}

.tab-filter-dark :deep(.ant-tabs-nav) {
  border-bottom-color: var(--border-color);
}

.tab-filter-dark :deep(.ant-tabs-tab.ant-tabs-tab-active) {
  background: transparent;
}

.tab-filter-dark :deep(.ant-tabs-tab) .tab-content {
  color: #ffffff;
}

.tab-filter-dark :deep(.ant-tabs-tab.ant-tabs-tab-active) .tab-content {
  color: #ffffff;
}

.tab-filter-dark :deep(.ant-tabs-tab.ant-tabs-tab-disabled) .tab-content {
  color: #666666;
}

/* Responsive Design */
/* Tablet View (≤768px) */
@media screen and (max-width: 768px) {
  .tab-filter :deep(.ant-tabs-nav) {
    padding: 0 8px;
  }

  .tab-filter :deep(.ant-tabs-tab) {
    padding: 10px 8px;
  }

  .tab-filter .tab-content {
    font-size: 13px;
    gap: 4px;
  }

  .tab-filter .tab-icon-wrapper {
    font-size: 18px;
  }
}

/* Mobile View (≤480px) */
@media screen and (max-width: 480px) {
  .tab-filter :deep(.ant-tabs-nav) {
    padding: 0 4px;
  }

  .tab-filter :deep(.ant-tabs-tab) {
    padding: 8px 6px;
    min-width: 44px;
  }

  .tab-filter .tab-content {
    justify-content: center;
  }

  .tab-filter .tab-icon-wrapper {
    font-size: 16px;
  }
}
</style>
