<script setup lang="ts">
import type { SiteInfo } from '@/api/site_navigation'
import { GlobalOutlined } from '@ant-design/icons-vue'
import VueDraggable from 'vuedraggable'
import { siteNavigationApi } from '@/api/site_navigation'
import { useWebSocket } from '@/lib/websocket'
import SiteCard from './components/SiteCard.vue'
import SiteHealthCheckModal from './components/SiteHealthCheckModal.vue'
import SiteNavigationToolbar from './components/SiteNavigationToolbar.vue'

const sites = ref<SiteInfo[]>([])
const { message } = useGlobalApp()
const loading = ref(true)
const refreshing = ref(false)
const settingsMode = ref(false)
const draggableSites = ref<SiteInfo[]>([])
const configModalVisible = ref(false)
const configTarget = ref<SiteInfo>()

watch(sites, newSites => {
  if (!settingsMode.value) {
    draggableSites.value = [...newSites]
  }
}, { immediate: true })

const { status, data, send, close } = useWebSocket(siteNavigationApi.websocketUrl)
const isConnected = computed(() => status.value === 'OPEN')

function hasCustomOrdering(siteList: SiteInfo[]): boolean {
  return siteList.some(site => Number.isFinite(site.custom_order) && site.custom_order !== 0)
}

function sortSitesByName(siteList: SiteInfo[]): SiteInfo[] {
  return [...siteList].sort((a, b) => {
    const nameCompare = (a.name || '').localeCompare(b.name || '', undefined, { sensitivity: 'base' })
    if (nameCompare !== 0) {
      return nameCompare
    }

    const fallbackA = a.display_url || a.host
    const fallbackB = b.display_url || b.host
    return fallbackA.localeCompare(fallbackB, undefined, { sensitivity: 'base' })
  })
}

function normalizeSites(siteList: SiteInfo[]): SiteInfo[] {
  if (siteList.length === 0) {
    return []
  }

  if (hasCustomOrdering(siteList)) {
    return [...siteList]
  }

  return sortSitesByName(siteList)
}

watch(data, newData => {
  if (newData.type === 'initial' || newData.type === 'update') {
    sites.value = normalizeSites(newData.data || [])
  }
})

async function loadSites() {
  try {
    loading.value = true
    const response = await siteNavigationApi.getSites()
    sites.value = normalizeSites(response.data || [])
  }
  catch (error) {
    console.error('Failed to load sites:', error)
  }
  finally {
    loading.value = false
  }
}

async function handleRefresh() {
  try {
    refreshing.value = true
    send(JSON.stringify({ type: 'refresh' }))
    message.success($gettext('Site refresh initiated'))
  }
  catch (error) {
    console.error('Failed to refresh sites:', error)
    message.error($gettext('Failed to refresh sites'))
  }
  finally {
    refreshing.value = false
  }
}

function toggleSettingsMode() {
  settingsMode.value = !settingsMode.value
  if (settingsMode.value) {
    draggableSites.value = [...sites.value]
  }
}

async function saveOrder() {
  try {
    const orderedIds = draggableSites.value.map(site => site.id)
    await siteNavigationApi.updateOrder(orderedIds)
    message.success($gettext('Order saved successfully'))
    sites.value = [...draggableSites.value]
    settingsMode.value = false
  }
  catch (error) {
    console.error('Failed to save order:', error)
    message.error($gettext('Failed to save order'))
  }
}

function cancelSettingsMode() {
  draggableSites.value = [...sites.value]
  settingsMode.value = false
}

function openConfigModal(site: SiteInfo) {
  configTarget.value = site
  configModalVisible.value = true
}

async function handleConfigSave(config: import('@/api/site_navigation').HealthCheckConfig) {
  try {
    if (configTarget.value) {
      await siteNavigationApi.updateHealthCheck(configTarget.value.id, config)
      message.success($gettext('Health check configuration saved'))
    }
  }
  catch (error) {
    console.error('Failed to save health check config:', error)
    message.error($gettext('Failed to save configuration'))
  }
}

const mounted = ref(false)

onMounted(async () => {
  await loadSites()
  mounted.value = true
})

onUnmounted(() => {
  close()
})
</script>

<template>
  <div class="site-navigation">
    <Teleport v-if="mounted" to=".action">
      <SiteNavigationToolbar
        :is-connected="isConnected"
        :refreshing="refreshing"
        :settings-mode="settingsMode"
        @refresh="handleRefresh"
        @toggle-settings="toggleSettingsMode"
        @save-order="saveOrder"
        @cancel-settings="cancelSettingsMode"
      />
    </Teleport>

    <div v-if="loading" class="flex items-center justify-center py-12">
      <ASpin size="large" />
    </div>

    <div v-else-if="draggableSites.length === 0" class="empty-state">
      <GlobalOutlined class="text-6xl text-gray-400 mb-4" />
      <h3 class="text-lg font-medium text-gray-900 dark:text-gray-100 mb-2">
        {{ $gettext('No sites found') }}
      </h3>
      <p class="text-gray-600 dark:text-gray-400 text-center max-w-md">
        {{ $gettext('Sites will appear here once you configure nginx server blocks with valid server_name directives.') }}
      </p>
    </div>

    <VueDraggable
      v-else
      v-model="draggableSites"
      :disabled="!settingsMode"
      class="site-grid"
      item-key="id"
      :animation="150"
      ghost-class="site-card-ghost"
      chosen-class="site-card-chosen"
      drag-class="site-card-drag"
    >
      <template #item="{ element }">
        <SiteCard
          :site="element"
          :settings-mode="settingsMode"
          @open-config="openConfigModal"
        />
      </template>
    </VueDraggable>

    <SiteHealthCheckModal
      v-model:open="configModalVisible"
      :site="configTarget"
      @save="handleConfigSave"
      @refresh="handleRefresh"
    />
  </div>
</template>

<style scoped>
.empty-state {
  @apply flex flex-col items-center justify-center py-16 text-center;
}

.site-grid {
  @apply grid gap-6;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
}

/* Responsive design for narrow screens */
@media (max-width: 768px) {
  .site-navigation {
    @apply p-4;
  }

  .site-grid {
    grid-template-columns: 1fr;
    @apply gap-4;
  }
}

@media (max-width: 480px) {
  .site-navigation {
    @apply p-3;
  }

  .site-grid {
    @apply gap-3;
  }
}
</style>
