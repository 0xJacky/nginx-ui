<script setup lang="ts">
import type { InstalledFilter, PluginDrawerTab } from './presets'
import type { PluginInfo } from '@/api/plugin'
import { AppstoreOutlined, ReloadOutlined, SearchOutlined, ShopOutlined, UploadOutlined } from '@antdv-next/icons'
import pluginApi, { localizedPluginName } from '@/api/plugin'
import gettext from '@/gettext'
import { getErrorMessage } from '@/lib/http'
import { usePluginLoader } from '@/plugin'
import InstalledPluginCard from './InstalledPluginCard.vue'
import { usePluginInventory } from './inventory'
import PermissionApprovalModal from './PermissionApprovalModal.vue'
import PluginDrawer from './PluginDrawer.vue'
import { matchesFilter, matchesKeyword, needsAttention } from './presets'
import SyncNodesModal from './SyncNodesModal.vue'

const emit = defineEmits<{
  install: []
  browse: []
}>()

const filter = defineModel<InstalledFilter>('filter', { default: 'all' })

const route = useRoute()
const router = useRouter()
const { message, modal } = useGlobalApp()
const inventory = usePluginInventory()
const { plugins, loading, error, hasNodes } = inventory
const pluginLoader = usePluginLoader()

const keyword = ref('')
const togglingId = ref('')
const approving = ref(false)

// The drawer follows the list, so a reload never shows stale data in it.
const selectedId = ref('')
const selected = computed(() => plugins.value.find(item => item.id === selectedId.value))
const drawerOpen = ref(false)
const drawerTab = ref<PluginDrawerTab>('overview')
const syncOpen = ref(false)
const approvalOpen = ref(false)
const pendingApproval = ref<PluginInfo>()

const counts = computed<Record<InstalledFilter, number>>(() => ({
  all: plugins.value.length,
  enabled: plugins.value.filter(item => item.enabled).length,
  disabled: plugins.value.filter(item => !item.enabled).length,
  attention: plugins.value.filter(needsAttention).length,
}))

const filterOptions = computed(() => [
  { value: 'all', label: $gettext('All') },
  { value: 'enabled', label: $gettext('Enabled') },
  { value: 'disabled', label: $gettext('Disabled') },
  { value: 'attention', label: $gettext('Needs attention') },
])

const visible = computed(() => plugins.value.filter(item =>
  matchesFilter(item, filter.value) && matchesKeyword(item, keyword.value)))

const isEmptyInventory = computed(() => !loading.value && !error.value && plugins.value.length === 0)

function openDrawer(plugin: PluginInfo, tab: PluginDrawerTab) {
  selectedId.value = plugin.id
  drawerTab.value = tab
  drawerOpen.value = true
}

// The open drawer lives in the URL, so a link can point straight at one plugin.
watch(drawerOpen, open => {
  const { plugin: _plugin, ...rest } = route.query
  router.replace({ query: open ? { ...rest, plugin: selectedId.value } : rest })
})

// A link may point straight at one plugin. It is honoured once the list is
// there, and again whenever the query changes while the page stays mounted.
watch([plugins, () => route.query.plugin], ([list, id]) => {
  if (typeof id !== 'string' || list.length === 0)
    return
  if (drawerOpen.value && selectedId.value === id)
    return

  const plugin = list.find(item => item.id === id)
  if (plugin)
    openDrawer(plugin, 'overview')
}, { immediate: true })

function openSync(plugin: PluginInfo) {
  selectedId.value = plugin.id
  syncOpen.value = true
}

async function enablePlugin(plugin: PluginInfo, approvePermissions?: boolean) {
  togglingId.value = plugin.id
  try {
    await pluginApi.enable(plugin.id, approvePermissions)
    message.success($gettext('Plugin enabled'))
    await inventory.reload(true)
    await pluginLoader.loadNew()
  }
  catch (e) {
    message.error(getErrorMessage(e, $gettext('Failed to enable the plugin')))
    await inventory.reload(true)
  }
  finally {
    togglingId.value = ''
  }
}

async function disablePlugin(plugin: PluginInfo) {
  togglingId.value = plugin.id
  try {
    await pluginApi.disable(plugin.id)
    message.success($gettext('Plugin disabled'))
    await inventory.reload(true)
  }
  catch (e) {
    message.error(getErrorMessage(e, $gettext('Failed to disable the plugin')))
    await inventory.reload(true)
  }
  finally {
    togglingId.value = ''
  }
}

function toggle(plugin: PluginInfo, checked: boolean) {
  if (!checked) {
    void disablePlugin(plugin)
    return
  }

  // Enabling a plugin whose permissions are not approved yet has to go through
  // the approval dialog first.
  if (plugin.status === 'needs_approval') {
    pendingApproval.value = plugin
    approvalOpen.value = true
    return
  }

  void enablePlugin(plugin)
}

async function approvePermissions() {
  const plugin = pendingApproval.value
  if (!plugin)
    return

  approving.value = true
  try {
    await enablePlugin(plugin, true)
    approvalOpen.value = false
    pendingApproval.value = undefined
  }
  finally {
    approving.value = false
  }
}

function confirmUninstall(plugin: PluginInfo) {
  modal.confirm({
    title: $gettext('Uninstall %{name}?', { name: localizedPluginName(plugin, gettext.current) }),
    content: $gettext('Its files, data and settings are removed from this node. Other nodes are not touched.'),
    okText: $gettext('Uninstall'),
    okButtonProps: { danger: true },
    cancelText: $gettext('Cancel'),
    async onOk() {
      try {
        await pluginApi.uninstall(plugin.id)
        message.success($gettext('Plugin uninstalled'))
        drawerOpen.value = false
        await inventory.reload(true)
      }
      catch (e) {
        message.error(getErrorMessage(e, $gettext('Failed to uninstall the plugin')))
      }
    },
  })
}
</script>

<template>
  <div>
    <div class="installed-toolbar">
      <AInput
        v-model:value="keyword"
        class="installed-search"
        :placeholder="$gettext('Search installed plugins')"
        allow-clear
      >
        <template #prefix>
          <SearchOutlined class="text-gray-400" />
        </template>
      </AInput>

      <div class="installed-filter">
        <ASegmented v-model:value="filter" :options="filterOptions">
          <template #labelRender="option">
            <span class="filter-label">
              {{ option.label }}
              <span class="filter-count">{{ counts[option.value as InstalledFilter] }}</span>
            </span>
          </template>
        </ASegmented>
      </div>

      <AButton class="ml-auto" :loading="loading" @click="inventory.reload()">
        <template #icon>
          <ReloadOutlined />
        </template>
        {{ $gettext('Refresh') }}
      </AButton>
    </div>

    <AAlert
      v-if="error"
      type="error"
      show-icon
      class="mb-4"
      :title="error"
    />

    <ASpin :spinning="loading">
      <div v-if="isEmptyInventory" class="installed-empty">
        <span class="installed-empty-icon">
          <AppstoreOutlined />
        </span>
        <h3 class="installed-empty-title">
          {{ $gettext('No plugins installed yet') }}
        </h3>
        <p class="installed-empty-text">
          {{ $gettext('Pick one from the marketplace or upload a bundle you built yourself.') }}
        </p>
        <div class="flex flex-wrap justify-center gap-2">
          <AButton type="primary" @click="emit('browse')">
            <template #icon>
              <ShopOutlined />
            </template>
            {{ $gettext('Browse marketplace') }}
          </AButton>
          <AButton @click="emit('install')">
            <template #icon>
              <UploadOutlined />
            </template>
            {{ $gettext('Install from file') }}
          </AButton>
        </div>
      </div>

      <div v-else-if="visible.length > 0" class="plugin-card-grid">
        <InstalledPluginCard
          v-for="plugin in visible"
          :key="plugin.id"
          :plugin="plugin"
          :has-nodes="hasNodes"
          :toggling="togglingId === plugin.id"
          @open="tab => openDrawer(plugin, tab)"
          @toggle="checked => toggle(plugin, checked)"
          @sync="openSync(plugin)"
          @uninstall="confirmUninstall(plugin)"
          @updated="inventory.reload(true)"
        />
      </div>

      <AEmpty
        v-else-if="!loading"
        class="py-8"
        :description="$gettext('No plugins match the current filter')"
      />
    </ASpin>

    <PluginDrawer
      v-model:open="drawerOpen"
      v-model:tab="drawerTab"
      :plugin="selected"
      :has-nodes="hasNodes"
      :toggling="Boolean(selected) && togglingId === selected?.id"
      @toggle="checked => selected && toggle(selected, checked)"
      @sync="syncOpen = true"
      @uninstall="selected && confirmUninstall(selected)"
      @updated="inventory.reload(true)"
    />
    <SyncNodesModal
      v-model:open="syncOpen"
      :plugin="selected"
      @synced="inventory.reload(true)"
    />
    <PermissionApprovalModal
      v-model:open="approvalOpen"
      :plugin="pendingApproval"
      :confirm-loading="approving"
      @approve="approvePermissions"
    />
  </div>
</template>

<style lang="less" scoped>
@import './plugin-card.less';

.installed-toolbar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 12px;
  margin-bottom: 16px;
}

.installed-search {
  width: 100%;
  max-width: 280px;
}

// A phone cannot fit every filter, so the control scrolls instead of clipping.
.installed-filter {
  max-width: 100%;
  overflow-x: auto;
}

.filter-label {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.filter-count {
  padding: 0 6px;
  font-size: 11px;
  font-variant-numeric: tabular-nums;
  line-height: 16px;
  color: var(--ant-color-text-secondary);
  background: var(--ant-color-fill-secondary);
  border-radius: 999px;
}

.installed-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 48px 16px;
  text-align: center;
  border: 1px dashed var(--ant-color-border);
  border-radius: var(--ant-border-radius-lg);
}

.installed-empty-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 56px;
  height: 56px;
  margin-bottom: 16px;
  font-size: 26px;
  color: var(--ant-color-primary);
  background: var(--ant-color-primary-bg);
  border-radius: 16px;
}

.installed-empty-title {
  margin: 0 0 6px;
  font-size: 16px;
  font-weight: 600;
}

.installed-empty-text {
  max-width: 420px;
  margin: 0 0 20px;
  color: var(--ant-color-text-secondary);
}
</style>
