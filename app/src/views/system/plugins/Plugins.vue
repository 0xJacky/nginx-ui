<script setup lang="ts">
import type { LocationQueryValue } from 'vue-router'
import type { InstalledFilter } from './presets'
import {
  AppstoreOutlined,
  ArrowUpOutlined,
  CheckCircleOutlined,
  ExclamationCircleOutlined,
  UploadOutlined,
} from '@antdv-next/icons'
import { usePluginLoader } from '@/plugin'
import DevPluginPopover from './DevPluginPopover.vue'
import InstalledList from './InstalledList.vue'
import InstallModal from './InstallModal.vue'
import { providePluginInventory } from './inventory'
import Marketplace from './Marketplace.vue'
import PluginMatrix from './PluginMatrix.vue'
import { needsAttention } from './presets'

type TabKey = 'installed' | 'marketplace' | 'nodes'

const tabKeys: TabKey[] = ['installed', 'marketplace', 'nodes']

const route = useRoute()
const router = useRouter()
const inventory = providePluginInventory()
const pluginLoader = usePluginLoader()

function readTab(value: LocationQueryValue | LocationQueryValue[] | undefined): TabKey {
  const key = Array.isArray(value) ? value[0] : value
  return tabKeys.includes(key as TabKey) ? key as TabKey : 'installed'
}

const activeKey = ref<TabKey>(readTab(route.query.tab))
const installedFilter = ref<InstalledFilter>('all')
const updatesOnly = ref(false)
const installOpen = ref(false)

// The tab lives in the URL so a reload or a shared link lands on the same view.
watch(activeKey, key => {
  router.replace({ query: { ...route.query, tab: key } })
})

const counts = computed(() => ({
  installed: inventory.plugins.value.length,
  enabled: inventory.plugins.value.filter(item => item.enabled).length,
  attention: inventory.plugins.value.filter(needsAttention).length,
  updates: inventory.updates.value.length,
}))

function showInstalled(filter: InstalledFilter) {
  installedFilter.value = filter
  activeKey.value = 'installed'
}

function showUpdates() {
  updatesOnly.value = counts.value.updates > 0
  activeKey.value = 'marketplace'
}

const stats = computed(() => [
  {
    key: 'installed',
    label: $gettext('Installed'),
    value: counts.value.installed,
    icon: AppstoreOutlined,
    tone: 'default',
    onClick: () => showInstalled('all'),
  },
  {
    key: 'enabled',
    label: $gettext('Enabled'),
    value: counts.value.enabled,
    icon: CheckCircleOutlined,
    tone: counts.value.enabled > 0 ? 'success' : 'default',
    onClick: () => showInstalled('enabled'),
  },
  {
    key: 'attention',
    label: $gettext('Needs attention'),
    value: counts.value.attention,
    icon: ExclamationCircleOutlined,
    tone: counts.value.attention > 0 ? 'warning' : 'default',
    onClick: () => showInstalled('attention'),
  },
  {
    key: 'updates',
    label: $gettext('Updates available'),
    value: counts.value.updates,
    icon: ArrowUpOutlined,
    tone: counts.value.updates > 0 ? 'info' : 'default',
    onClick: showUpdates,
  },
])

const tabs = computed(() => [
  { key: 'installed', label: $gettext('Installed'), count: counts.value.installed },
  { key: 'marketplace', label: $gettext('Marketplace'), count: counts.value.updates },
  { key: 'nodes', label: $gettext('Nodes') },
])

// A freshly installed bundle is picked up without a page reload.
async function onInstalled() {
  await inventory.reload(true)
  await Promise.all([inventory.reloadUpdates(), pluginLoader.loadNew()])
}
</script>

<template>
  <ACard class="plugins-page">
    <div class="plugins-header">
      <div class="plugins-heading">
        <h2 class="plugins-title">
          {{ $gettext('Plugins') }}
        </h2>
        <p class="plugins-subtitle">
          {{ $gettext('Plugins extend Nginx UI with extra capabilities. Only install bundles from authors you trust.') }}
        </p>
      </div>

      <div class="plugins-actions">
        <DevPluginPopover />
        <AButton type="primary" @click="installOpen = true">
          <template #icon>
            <UploadOutlined />
          </template>
          {{ $gettext('Install from file') }}
        </AButton>
      </div>
    </div>

    <div class="plugins-stats">
      <button
        v-for="stat in stats"
        :key="stat.key"
        type="button"
        class="stat-tile"
        :class="`is-${stat.tone}`"
        @click="stat.onClick"
      >
        <span class="stat-icon">
          <component :is="stat.icon" />
        </span>
        <span class="stat-body">
          <span class="stat-value">{{ stat.value }}</span>
          <span class="stat-label">{{ stat.label }}</span>
        </span>
      </button>
    </div>

    <ATabs v-model:active-key="activeKey" :items="tabs">
      <template #labelRender="{ item }">
        <span class="tab-label">
          {{ item.label }}
          <span v-if="item.count" class="tab-count">{{ item.count }}</span>
        </span>
      </template>

      <template #contentRender="{ item }">
        <InstalledList
          v-if="item.key === 'installed'"
          v-model:filter="installedFilter"
          @install="installOpen = true"
          @browse="activeKey = 'marketplace'"
        />
        <Marketplace
          v-else-if="item.key === 'marketplace'"
          v-model:updates-only="updatesOnly"
        />
        <PluginMatrix v-else-if="item.key === 'nodes'" />
      </template>
    </ATabs>

    <InstallModal v-model:open="installOpen" @installed="onInstalled" />
  </ACard>
</template>

<style lang="less" scoped>
.plugins-header {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px 24px;
}

.plugins-heading {
  min-width: 0;
  max-width: 640px;
}

.plugins-title {
  margin: 0 0 4px;
  font-size: 20px;
  font-weight: 600;
  line-height: 1.4;
}

.plugins-subtitle {
  margin: 0;
  line-height: 1.6;
  color: var(--ant-color-text-secondary);
}

.plugins-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.plugins-stats {
  display: grid;
  gap: 12px;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  margin: 20px 0 8px;

  @media (min-width: 960px) {
    grid-template-columns: repeat(4, minmax(0, 1fr));
  }
}

.stat-tile {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
  padding: 14px 16px;
  font: inherit;
  text-align: left;
  color: var(--ant-color-text);
  background: var(--ant-color-bg-container);
  border: 1px solid var(--ant-color-border-secondary);
  border-radius: var(--ant-border-radius-lg);
  cursor: pointer;
  transition: border-color 0.2s ease;

  &:hover {
    border-color: var(--ant-color-primary-border);
  }

  &:focus-visible {
    outline: 2px solid var(--ant-color-primary);
    outline-offset: 2px;
  }
}

.stat-icon {
  flex: none;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 40px;
  height: 40px;
  font-size: 18px;
  color: var(--ant-color-text-secondary);
  background: var(--ant-color-fill-tertiary);
  border-radius: 12px;
}

.is-success .stat-icon {
  color: var(--ant-color-success);
  background: var(--ant-color-success-bg);
}

.is-warning .stat-icon {
  color: var(--ant-color-warning);
  background: var(--ant-color-warning-bg);
}

.is-info .stat-icon {
  color: var(--ant-color-primary);
  background: var(--ant-color-primary-bg);
}

.stat-body {
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.stat-value {
  font-size: 22px;
  font-variant-numeric: tabular-nums;
  font-weight: 600;
  line-height: 1.2;
}

.stat-label {
  font-size: 12px;
  line-height: 1.3;
  color: var(--ant-color-text-secondary);
}

.tab-label {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.tab-count {
  padding: 0 6px;
  font-size: 11px;
  font-variant-numeric: tabular-nums;
  line-height: 16px;
  color: var(--ant-color-text-secondary);
  background: var(--ant-color-fill-secondary);
  border-radius: 999px;
}

@media (prefers-reduced-motion: reduce) {
  .stat-tile {
    transition: none;
  }
}
</style>
