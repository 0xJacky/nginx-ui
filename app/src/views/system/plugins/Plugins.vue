<script setup lang="ts">
import type { MenuProps } from 'antdv-next'
import type { LocationQueryValue } from 'vue-router'
import type { InstalledFilter } from './presets'
import { EllipsisOutlined, ExperimentOutlined, UploadOutlined } from '@antdv-next/icons'
import settingsApi from '@/api/settings'
import { usePluginLoader, usePluginStore } from '@/plugin'
import DevPluginModal from './DevPluginModal.vue'
import InstalledList from './InstalledList.vue'
import InstallModal from './InstallModal.vue'
import { providePluginInventory } from './inventory'
import Marketplace from './Marketplace.vue'
import PluginMatrix from './PluginMatrix.vue'

type TabKey = 'installed' | 'marketplace' | 'nodes'

const tabKeys: TabKey[] = ['installed', 'marketplace', 'nodes']

const route = useRoute()
const router = useRouter()
const inventory = providePluginInventory()
const pluginLoader = usePluginLoader()
const pluginStore = usePluginStore()

function readTab(value: LocationQueryValue | LocationQueryValue[] | undefined): TabKey {
  const key = Array.isArray(value) ? value[0] : value
  return tabKeys.includes(key as TabKey) ? key as TabKey : 'installed'
}

const activeKey = ref<TabKey>(readTab(route.query.tab))
const installedFilter = ref<InstalledFilter>('all')
const updatesOnly = ref(false)
const installOpen = ref(false)
const devPluginOpen = ref(false)
const developerMode = ref(false)

// The tab lives in the URL so a reload or a shared link lands on the same view.
watch(activeKey, key => {
  router.replace({ query: { ...route.query, tab: key } })
})

// Developer tools only show in developer mode. A failed read keeps them hidden.
onMounted(async () => {
  try {
    const data = await settingsApi.get()
    developerMode.value = Boolean(data.plugin?.developer_mode)
  }
  catch {
    developerMode.value = false
  }
})

// A development URL set earlier stays reachable so it can be cleared.
const showDevPlugin = computed(() => developerMode.value || Boolean(pluginStore.devPluginUrl))

const moreItems = computed<MenuProps['items']>(() => {
  const items: NonNullable<MenuProps['items']> = []
  if (showDevPlugin.value)
    items.push({ key: 'dev', label: $gettext('Develop plugin'), icon: h(ExperimentOutlined) })
  return items
})

function onMoreClick({ key }: { key: string | number }) {
  if (key === 'dev')
    devPluginOpen.value = true
}

const tabs = computed(() => [
  { key: 'installed', label: $gettext('Installed'), count: inventory.plugins.value.length },
  { key: 'marketplace', label: $gettext('Marketplace'), count: inventory.updates.value.length },
  { key: 'nodes', label: $gettext('Nodes') },
])

// A freshly installed bundle is picked up without a page reload.
async function onInstalled() {
  await inventory.reload(true)
  await Promise.all([inventory.reloadUpdates(), inventory.reloadCatalog(), pluginLoader.loadNew()])
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
        <AButton type="primary" @click="installOpen = true">
          <template #icon>
            <UploadOutlined />
          </template>
          {{ $gettext('Install from file') }}
        </AButton>
        <ADropdown
          v-if="moreItems?.length"
          :trigger="['click']"
          placement="bottomRight"
          :menu="{ items: moreItems, onClick: onMoreClick }"
        >
          <AButton :aria-label="$gettext('More actions')">
            <template #icon>
              <EllipsisOutlined />
            </template>
          </AButton>
        </ADropdown>
      </div>
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
    <DevPluginModal v-model:open="devPluginOpen" />
  </ACard>
</template>

<style lang="less" scoped>
.plugins-header {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px 24px;
  margin-bottom: 8px;
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
</style>
