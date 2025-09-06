<script setup lang="ts">
import type ReconnectingWebSocket from 'reconnecting-websocket'
import type { SiteInfo } from '@/api/site_navigation'
import { GlobalOutlined } from '@ant-design/icons-vue'
import Sortable from 'sortablejs'
import { siteNavigationApi } from '@/api/site_navigation'
import SiteCard from './components/SiteCard.vue'
import SiteHealthCheckModal from './components/SiteHealthCheckModal.vue'
import SiteNavigationToolbar from './components/SiteNavigationToolbar.vue'

const sites = ref<SiteInfo[]>([])
const { message } = useGlobalApp()
const loading = ref(true)
const refreshing = ref(false)
const isConnected = ref(false)
const settingsMode = ref(false)
const draggableSites = ref<SiteInfo[]>([])
const configModalVisible = ref(false)
const configTarget = ref<SiteInfo>()

let sortableInstance: Sortable | null = null
let websocket: ReconnectingWebSocket | WebSocket | null = null

// Display sites - use draggable sites in settings mode, backend sorted sites otherwise
const displaySites = computed(() => {
  return settingsMode.value ? draggableSites.value : sites.value
})

// WebSocket connection
function connectWebSocket() {
  try {
    websocket = siteNavigationApi.createWebSocket()

    if (!websocket) {
      isConnected.value = false
      return
    }

    websocket.onopen = () => {
      isConnected.value = true
    }

    websocket.onmessage = (event: MessageEvent) => {
      try {
        const data = JSON.parse(event.data)
        if (data.type === 'initial' || data.type === 'update') {
          sites.value = data.data || []
        }
      }
      catch (error) {
        console.error('Failed to parse WebSocket message:', error)
      }
    }

    websocket.onclose = () => {
      isConnected.value = false
    }

    websocket.onerror = error => {
      console.error('Site navigation WebSocket error:', error)
      isConnected.value = false
    }
  }
  catch (error) {
    console.error('Failed to connect WebSocket:', error)
    isConnected.value = false
  }
}

// Load sites via HTTP (fallback)
async function loadSites() {
  try {
    loading.value = true
    const response = await siteNavigationApi.getSites()
    sites.value = response.data || []
  }
  catch (error) {
    console.error('Failed to load sites:', error)
  }
  finally {
    loading.value = false
  }
}

// Refresh sites
async function handleRefresh() {
  try {
    refreshing.value = true

    // Only use WebSocket refresh
    if (websocket && isConnected.value) {
      websocket.send(JSON.stringify({ type: 'refresh' }))
      message.success($gettext('Site refresh initiated'))
    }
    else {
      message.warning($gettext('WebSocket not connected, please wait for connection'))
    }
  }
  catch (error) {
    console.error('Failed to refresh sites:', error)
    message.error($gettext('Failed to refresh sites'))
  }
  finally {
    refreshing.value = false
  }
}

// Toggle settings mode
function toggleSettingsMode() {
  settingsMode.value = !settingsMode.value

  if (settingsMode.value) {
    draggableSites.value = [...sites.value]
    nextTick(() => initSortable())
  }
  else {
    destroySortable()
  }
}

// Initialize sortable
function initSortable() {
  const gridElement = document.querySelector('.site-grid')
  if (gridElement && !sortableInstance) {
    sortableInstance = new Sortable(gridElement as HTMLElement, {
      animation: 150,
      ghostClass: 'site-card-ghost',
      chosenClass: 'site-card-chosen',
      dragClass: 'site-card-drag',
      onEnd: () => {
        // Update draggableSites order based on DOM order
        const cards = Array.from(gridElement.children)
        const newOrder = cards.map(card => {
          const url = card.getAttribute('data-url')
          return draggableSites.value.find(site => site.url === url)!
        })
        draggableSites.value = newOrder
      },
    })
  }
}

// Destroy sortable
function destroySortable() {
  if (sortableInstance) {
    sortableInstance.destroy()
    sortableInstance = null
  }
}

// Save order
async function saveOrder() {
  try {
    const orderedIds = draggableSites.value.map(site => site.id)
    await siteNavigationApi.updateOrder(orderedIds)
    message.success($gettext('Order saved successfully'))

    // Update sites.value immediately to reflect the new order
    sites.value = [...draggableSites.value]

    settingsMode.value = false
    destroySortable()
  }
  catch (error) {
    console.error('Failed to save order:', error)
    message.error($gettext('Failed to save order'))
  }
}

// Cancel settings mode
function cancelSettingsMode() {
  settingsMode.value = false
  destroySortable()
  draggableSites.value = []
}

// Open config modal
function openConfigModal(site: SiteInfo) {
  configTarget.value = site
  configModalVisible.value = true
}

// Handle health check config save
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

onMounted(async () => {
  // First load data via HTTP
  await loadSites()
  // Then connect WebSocket for real-time updates
  connectWebSocket()
})

onUnmounted(() => {
  destroySortable()
  if (websocket) {
    websocket.close()
  }
})
</script>

<template>
  <div class="site-navigation">
    <SiteNavigationToolbar
      :is-connected="isConnected"
      :refreshing="refreshing"
      :settings-mode="settingsMode"
      @refresh="handleRefresh"
      @toggle-settings="toggleSettingsMode"
      @save-order="saveOrder"
      @cancel-settings="cancelSettingsMode"
    />

    <div v-if="loading" class="flex items-center justify-center py-12">
      <ASpin size="large" />
    </div>

    <div v-else-if="displaySites.length === 0" class="empty-state">
      <GlobalOutlined class="text-6xl text-gray-400 mb-4" />
      <h3 class="text-lg font-medium text-gray-900 dark:text-gray-100 mb-2">
        {{ $gettext('No sites found') }}
      </h3>
      <p class="text-gray-600 dark:text-gray-400 text-center max-w-md">
        {{ $gettext('Sites will appear here once you configure nginx server blocks with valid server_name directives.') }}
      </p>
    </div>

    <div v-else class="site-grid">
      <SiteCard
        v-for="site in displaySites"
        :key="site.id"
        :site="site"
        :settings-mode="settingsMode"
        @open-config="openConfigModal"
      />
    </div>

    <SiteHealthCheckModal
      v-model:open="configModalVisible"
      :site="configTarget"
      @save="handleConfigSave"
      @refresh="handleRefresh"
    />
  </div>
</template>

<style scoped>
.site-navigation {
  @apply p-6;
}

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
