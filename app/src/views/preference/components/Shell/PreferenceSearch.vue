<script setup lang="ts">
import type { SettingCatalogEntry } from '../../catalog'
import type { PreferenceSection } from '../../sections'
import { SearchOutlined } from '@antdv-next/icons'

const props = defineProps<{
  entries: SettingCatalogEntry[]
  sections: PreferenceSection[]
}>()

const emit = defineEmits<{
  select: [entry: SettingCatalogEntry]
}>()

interface SearchOption {
  value: string
  title: string
  description: string
  sectionLabel: string
}

const MAX_RESULTS = 12

const query = ref('')

const sectionLabels = computed(() => Object.fromEntries(
  props.sections.map(section => [section.key, section.label]),
))

const visibleEntries = computed(() => {
  const visibleKeys = new Set(props.sections.map(section => section.key))
  return props.entries.filter(entry => visibleKeys.has(entry.section))
})

function matches(entry: SettingCatalogEntry, keyword: string) {
  return entry.title.toLowerCase().includes(keyword)
    || (entry.description?.toLowerCase().includes(keyword) ?? false)
    || (entry.panel?.toLowerCase().includes(keyword) ?? false)
}

const options = computed<SearchOption[]>(() => {
  const keyword = query.value.trim().toLowerCase()
  if (!keyword)
    return []

  // Title hits come first so the most likely match is at the top.
  const byTitle = visibleEntries.value.filter(entry => entry.title.toLowerCase().includes(keyword))
  const byText = visibleEntries.value.filter(entry => !byTitle.includes(entry) && matches(entry, keyword))

  return [...byTitle, ...byText].slice(0, MAX_RESULTS).map(entry => ({
    value: entry.path,
    title: entry.panel ? `${entry.panel} · ${entry.title}` : entry.title,
    description: entry.description ?? '',
    sectionLabel: sectionLabels.value[entry.section] ?? '',
  }))
})

function onSelect(value: unknown) {
  const entry = props.entries.find(item => item.path === value)
  if (entry)
    emit('select', entry)
  nextTick(() => {
    query.value = ''
  })
}
</script>

<template>
  <AAutoComplete
    v-model:value="query"
    :options="options"
    :filter-option="false"
    default-active-first-option
    :popup-match-select-width="360"
    class="preference-search"
    @select="onSelect"
  >
    <AInput
      :placeholder="$gettext('Search settings')"
      allow-clear
    >
      <template #prefix>
        <SearchOutlined class="text-gray-400" />
      </template>
    </AInput>
    <template #optionRender="{ option }">
      <div class="preference-search-option">
        <div class="preference-search-option-head">
          <span class="preference-search-option-title">{{ option.data.title }}</span>
          <span class="preference-search-option-section">{{ option.data.sectionLabel }}</span>
        </div>
        <div
          v-if="option.data.description"
          class="preference-search-option-desc"
        >
          {{ option.data.description }}
        </div>
      </div>
    </template>
    <template #notFoundContent>
      <div class="preference-search-empty">
        {{ $gettext('No matching settings') }}
      </div>
    </template>
  </AAutoComplete>
</template>

<style lang="less" scoped>
.preference-search {
  width: 280px;
  max-width: 100%;
}

.preference-search-option {
  display: flex;
  flex-direction: column;
  gap: 2px;
  white-space: normal;
}

.preference-search-option-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 12px;
}

.preference-search-option-title {
  font-weight: 500;
}

.preference-search-option-section {
  flex: none;
  font-size: 12px;
  color: var(--ant-color-text-tertiary);
}

.preference-search-option-desc {
  font-size: 12px;
  line-height: 1.5;
  color: var(--ant-color-text-secondary);
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.preference-search-empty {
  padding: 4px 0;
  font-size: 13px;
  color: var(--ant-color-text-tertiary);
}

@media (max-width: 600px) {
  .preference-search {
    width: 100%;
  }
}
</style>
