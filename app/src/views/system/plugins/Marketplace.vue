<script setup lang="ts">
import type { SelectProps } from 'antdv-next'
import type { CatalogEntry, PluginUpdateInfo } from '@/api/plugin_marketplace'
import { ReloadOutlined, SearchOutlined, SettingOutlined } from '@antdv-next/icons'
import { refDebounced } from '@vueuse/core'
import { getMarketplaceList, getPluginUpdates } from '@/api/plugin_marketplace'
import { getErrorMessage } from '@/lib/http'
import InstallConfirmModal from './marketplace/InstallConfirmModal.vue'
import PluginCard from './marketplace/PluginCard.vue'
import PluginDetailDrawer from './marketplace/PluginDetailDrawer.vue'
import SourcesModal from './marketplace/SourcesModal.vue'

const { message } = App.useApp()

const loading = ref(false)
const error = ref('')
const entries = ref<CatalogEntry[]>([])
const sources = ref<string[]>([])
const updates = ref<PluginUpdateInfo[]>([])

const keyword = ref('')
const debouncedKeyword = refDebounced(keyword, 300)
const category = ref<string>()
const source = ref<string>()
const updatesOnly = ref(false)

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

const visibleEntries = computed(() => {
  if (!updatesOnly.value)
    return entries.value
  return entries.value.filter(entry => entry.update_available)
})

const updateCount = computed(() => updates.value.length)

async function loadUpdates() {
  try {
    updates.value = await getPluginUpdates()
  }
  catch {
    // The badge is a convenience, a failed probe must not break the grid.
    updates.value = []
  }
}

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
  await Promise.all([load(refresh), loadUpdates()])
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

async function onInstalled() {
  await refreshAll(false)
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
    <div class="mb-4 flex flex-wrap items-center gap-2">
      <AInput
        v-model:value="keyword"
        class="marketplace-search"
        :placeholder="$gettext('Search plugins')"
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

      <div class="flex items-center gap-2">
        <ASwitch v-model:checked="updatesOnly" size="small" />
        <span class="text-sm">
          {{ $gettext('Updates only') }}
          <ATag v-if="updateCount > 0" color="orange" class="ml-1">{{ updateCount }}</ATag>
        </span>
      </div>

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

    <AAlert
      v-if="error"
      type="error"
      show-icon
      class="mb-4"
      :title="error"
    />

    <ASpin :spinning="loading">
      <div v-if="visibleEntries.length > 0" class="marketplace-grid">
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
        :description="updatesOnly
          ? $gettext('Every installed plugin is up to date')
          : $gettext('No plugins match your search')"
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
.marketplace-search {
  width: 100%;
  max-width: 260px;
}

.marketplace-select {
  width: 100%;
  max-width: 180px;
  min-width: 140px;
}

.marketplace-grid {
  display: grid;
  gap: 16px;
  grid-template-columns: 1fr;
}

@media (min-width: 640px) {
  .marketplace-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (min-width: 1200px) {
  .marketplace-grid {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}
</style>
