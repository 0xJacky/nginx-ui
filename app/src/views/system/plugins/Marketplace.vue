<script setup lang="ts">
import type { SelectProps } from 'antdv-next'
import type { CatalogEntry } from '@/api/plugin_marketplace'
import { ArrowUpOutlined, DisconnectOutlined, ReloadOutlined, SearchOutlined, SettingOutlined } from '@antdv-next/icons'
import { refDebounced } from '@vueuse/core'
import { getMarketplaceList } from '@/api/plugin_marketplace'
import { getErrorMessage } from '@/lib/http'
import { usePluginLoader } from '@/plugin'
import { usePluginInventory } from './inventory'
import InstallConfirmModal from './marketplace/InstallConfirmModal.vue'
import PluginCard from './marketplace/PluginCard.vue'
import PluginDetailDrawer from './marketplace/PluginDetailDrawer.vue'
import SourcesModal from './marketplace/SourcesModal.vue'

const updatesOnly = defineModel<boolean>('updatesOnly', { default: false })

const { message } = useGlobalApp()
const inventory = usePluginInventory()
const pluginLoader = usePluginLoader()

const loading = ref(false)
const error = ref('')
const entries = ref<CatalogEntry[]>([])
const sources = ref<string[]>([])

const keyword = ref('')
const debouncedKeyword = refDebounced(keyword, 300)
const category = ref<string>()
const source = ref<string>()

const detailOpen = ref(false)
const installOpen = ref(false)
const sourcesOpen = ref(false)
const selected = ref<CatalogEntry>()

const categoryOptions = computed<SelectProps['options']>(() => {
  const seen = new Set<string>()
  entries.value.forEach(entry => entry.categories?.forEach(item => seen.add(item)))
  return [...seen].sort().map(item => ({ value: item, label: item }))
})

const sourceOptions = computed<SelectProps['options']>(() =>
  sources.value.map(item => ({ value: item, label: item })))

const updateCount = computed(() => inventory.updates.value.length)

const scopeOptions = computed(() => [
  { value: 'all', label: $gettext('All plugins') },
  { value: 'updates', label: $gettext('Updates') },
])

const scope = computed({
  get: () => (updatesOnly.value ? 'updates' : 'all'),
  set: (value: string) => {
    updatesOnly.value = value === 'updates'
  },
})

const visibleEntries = computed(() => {
  if (!updatesOnly.value)
    return entries.value
  return entries.value.filter(entry => entry.update_available)
})

const emptyText = computed(() => {
  if (updatesOnly.value)
    return $gettext('Every installed plugin is up to date')
  if (keyword.value || category.value || source.value)
    return $gettext('No plugins match your search')
  return $gettext('The catalog is empty')
})

async function load(refresh = false) {
  loading.value = true
  error.value = ''
  try {
    const response = await getMarketplaceList({
      keyword: debouncedKeyword.value,
      category: category.value,
      source: source.value,
      refresh,
    })
    entries.value = response.plugins
    sources.value = response.sources
  }
  catch (e) {
    entries.value = []
    error.value = getErrorMessage(e, $gettext('Failed to load the plugin marketplace'))
  }
  finally {
    loading.value = false
  }
}

async function refreshAll(refresh = false) {
  await Promise.all([load(refresh), inventory.reloadUpdates()])
}

function openDetail(entry: CatalogEntry) {
  selected.value = entry
  detailOpen.value = true
}

function openInstall(entry: CatalogEntry) {
  selected.value = entry
  detailOpen.value = false
  installOpen.value = true
}

// The installed tab and the counters see the new plugin right away.
async function onInstalled() {
  await Promise.all([refreshAll(false), inventory.reload(true)])
  await pluginLoader.loadNew()
}

function onSourcesSaved() {
  message.success($gettext('Reloading the catalog'))
  void refreshAll(true)
}

watch([debouncedKeyword, category, source], () => load())
onMounted(() => refreshAll())
</script>

<template>
  <div>
    <div class="marketplace-toolbar">
      <AInput
        v-model:value="keyword"
        class="marketplace-search"
        :placeholder="$gettext('Search the marketplace')"
        allow-clear
      >
        <template #prefix>
          <SearchOutlined class="text-gray-400" />
        </template>
      </AInput>

      <ASelect
        v-model:value="category"
        class="marketplace-select"
        :options="categoryOptions"
        :placeholder="$gettext('All categories')"
        allow-clear
      />

      <ASelect
        v-if="sourceOptions && sourceOptions.length > 1"
        v-model:value="source"
        class="marketplace-select"
        :options="sourceOptions"
        :placeholder="$gettext('All sources')"
        allow-clear
      />

      <ASegmented v-model:value="scope" :options="scopeOptions">
        <template #labelRender="option">
          <span class="scope-label">
            {{ option.label }}
            <span v-if="option.value === 'updates' && updateCount > 0" class="scope-count">{{ updateCount }}</span>
          </span>
        </template>
      </ASegmented>

      <div class="ml-auto flex flex-wrap gap-2">
        <AButton :loading="loading" @click="refreshAll(true)">
          <template #icon>
            <ReloadOutlined />
          </template>
          {{ $gettext('Refresh') }}
        </AButton>
        <AButton @click="sourcesOpen = true">
          <template #icon>
            <SettingOutlined />
          </template>
          {{ $gettext('Sources') }}
        </AButton>
      </div>
    </div>

    <div v-if="updateCount > 0 && !updatesOnly" class="updates-banner">
      <span class="updates-banner-icon">
        <ArrowUpOutlined />
      </span>
      <span class="min-w-0 flex-1">
        {{ $ngettext('%{count} installed plugin has a newer release.', '%{count} installed plugins have a newer release.', updateCount, { count: String(updateCount) }) }}
      </span>
      <AButton size="small" type="primary" ghost @click="updatesOnly = true">
        {{ $gettext('Show updates') }}
      </AButton>
    </div>

    <ASpin :spinning="loading">
      <div v-if="error" class="marketplace-unavailable">
        <span class="marketplace-unavailable-icon">
          <DisconnectOutlined />
        </span>
        <h3 class="marketplace-unavailable-title">
          {{ $gettext('The marketplace is unavailable') }}
        </h3>
        <p class="marketplace-unavailable-text">
          {{ error }}
        </p>
        <div class="flex flex-wrap justify-center gap-2">
          <AButton type="primary" :loading="loading" @click="refreshAll(true)">
            <template #icon>
              <ReloadOutlined />
            </template>
            {{ $gettext('Retry') }}
          </AButton>
          <AButton @click="sourcesOpen = true">
            <template #icon>
              <SettingOutlined />
            </template>
            {{ $gettext('Sources') }}
          </AButton>
        </div>
      </div>

      <div v-else-if="visibleEntries.length > 0" class="plugin-card-grid">
        <PluginCard
          v-for="entry in visibleEntries"
          :key="`${entry.source}:${entry.id}`"
          :entry="entry"
          @detail="openDetail"
          @install="openInstall"
        />
      </div>
      <AEmpty
        v-else-if="!loading"
        class="py-8"
        :description="emptyText"
      />
    </ASpin>

    <PluginDetailDrawer
      v-model:open="detailOpen"
      :entry="selected"
      @install="openInstall"
    />
    <InstallConfirmModal
      v-model:open="installOpen"
      :entry="selected"
      @installed="onInstalled"
    />
    <SourcesModal v-model:open="sourcesOpen" @saved="onSourcesSaved" />
  </div>
</template>

<style lang="less" scoped>
@import './plugin-card.less';

.marketplace-toolbar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 12px;
  margin-bottom: 16px;
}

.marketplace-search {
  width: 100%;
  max-width: 280px;
}

.marketplace-select {
  width: 100%;
  max-width: 180px;
  min-width: 140px;
}

.scope-label {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.scope-count {
  padding: 0 6px;
  font-size: 11px;
  font-variant-numeric: tabular-nums;
  line-height: 16px;
  color: var(--ant-color-primary);
  background: var(--ant-color-primary-bg);
  border-radius: 999px;
}

.updates-banner {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 16px;
  padding: 10px 14px;
  color: var(--ant-color-text);
  background: var(--ant-color-primary-bg);
  border: 1px solid var(--ant-color-primary-border);
  border-radius: var(--ant-border-radius-lg);
}

.updates-banner-icon {
  flex: none;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  color: var(--ant-color-primary);
  background: var(--ant-color-bg-container);
  border-radius: 8px;
}

.marketplace-unavailable {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 48px 16px;
  text-align: center;
  border: 1px dashed var(--ant-color-border);
  border-radius: var(--ant-border-radius-lg);
}

.marketplace-unavailable-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 56px;
  height: 56px;
  margin-bottom: 16px;
  font-size: 26px;
  color: var(--ant-color-warning);
  background: var(--ant-color-warning-bg);
  border-radius: 16px;
}

.marketplace-unavailable-title {
  margin: 0 0 6px;
  font-size: 16px;
  font-weight: 600;
}

.marketplace-unavailable-text {
  max-width: 480px;
  margin: 0 0 20px;
  color: var(--ant-color-text-secondary);
  word-break: break-word;
}
</style>
