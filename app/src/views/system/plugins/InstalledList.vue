<script setup lang="ts">
import type { InstalledFilter, PluginDrawerTab } from './presets'
import type { PluginInfo, PluginUsage } from '@/api/plugin'
import type { CatalogEntry } from '@/api/plugin_marketplace'
import { AppstoreOutlined, ArrowUpOutlined, ReloadOutlined, SearchOutlined, ShopOutlined, UploadOutlined } from '@antdv-next/icons'
import pluginApi, { localizedPluginName } from '@/api/plugin'
import gettext from '@/gettext'
import { getErrorMessage } from '@/lib/http'
import { usePluginLoader } from '@/plugin'
import { enableReplacesText } from './conflicts'
import InstalledPluginCard from './InstalledPluginCard.vue'
import { usePluginInventory } from './inventory'
import InstallConfirmModal from './marketplace/InstallConfirmModal.vue'
import PluginDetailDrawer from './marketplace/PluginDetailDrawer.vue'
import PermissionApprovalModal from './PermissionApprovalModal.vue'
import PluginDrawer from './PluginDrawer.vue'
import { matchesFilter, matchesKeyword, needsAttention } from './presets'
import SyncNodesModal from './SyncNodesModal.vue'
import { fallbackCatalogEntry, findSingleUpdate } from './updates'
import { certificateUsageText, formatUsagePreview, previewUsage } from './usage'
import { enabledConflictNames } from './useConflicts'

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
// Set when the user already agreed to turn off the plugins that conflict.
const pendingReplace = ref(false)
const updateModalOpen = ref(false)
const updateDetailOpen = ref(false)
const updateTargetEntry = ref<CatalogEntry>()
const updateTargetId = ref<string>()
const updateTargetVersion = ref<string>()

const updateById = computed(() => new Map(inventory.updates.value.map(item => [item.id, item])))
const catalogById = computed(() => new Map(inventory.catalog.value.map(entry => [entry.id, entry])))
const updateIds = computed(() => new Set(updateById.value.keys()))
// Installed plugins the catalogs took down, by id.
const delistedById = computed(() => new Map(inventory.delisted.value.map(notice => [notice.id, notice])))
// Installed versions the catalog withdrew, a revoked signing key for one,
// also when the plugin was taken down since.
const withdrawnIds = computed(() => new Set([
  ...inventory.catalog.value
    .filter(entry => entry.installed_yanked && plugins.value.some(item => item.id === entry.id && item.version === entry.installed_version))
    .map(entry => entry.id),
  ...inventory.delisted.value.filter(notice => notice.withdrawn).map(notice => notice.id),
]))

const counts = computed<Record<InstalledFilter, number>>(() => ({
  all: plugins.value.length,
  enabled: plugins.value.filter(item => item.enabled).length,
  disabled: plugins.value.filter(item => !item.enabled).length,
  attention: plugins.value.filter(item => needsAttention(item, withdrawnIds.value.has(item.id))).length,
  updates: plugins.value.filter(item => updateIds.value.has(item.id)).length,
}))

const filterOptions = computed(() => {
  const options: { value: InstalledFilter, label: string }[] = [
    {
      value: 'all',
      label: $gettext('All'),
    },
    {
      value: 'enabled',
      label: $gettext('Enabled'),
    },
    {
      value: 'disabled',
      label: $gettext('Disabled'),
    },
    {
      value: 'attention',
      label: $gettext('Needs attention'),
    },
  ]
  if (counts.value.updates > 0) {
    options.push({
      value: 'updates',
      label: $gettext('Updates available'),
    })
  }
  return options
})

// The updates entry disappears once everything is current.
watch(() => counts.value.updates, count => {
  if (count === 0 && filter.value === 'updates')
    filter.value = 'all'
})

const visible = computed(() => plugins.value.filter(item =>
  matchesFilter(item, filter.value, updateIds.value, withdrawnIds.value) && matchesKeyword(item, keyword.value)))

const singleUpdate = computed(() => findSingleUpdate(plugins.value, inventory.updates.value))

const isEmptyInventory = computed(() => !loading.value && !error.value && plugins.value.length === 0)

function catalogEntryFor(plugin: PluginInfo): CatalogEntry | undefined {
  const existing = catalogById.value.get(plugin.id)
  if (existing)
    return existing
  const update = updateById.value.get(plugin.id)
  return update
    ? fallbackCatalogEntry(plugin, update)
    : undefined
}

function openUpdate(plugin: PluginInfo, version?: string) {
  updateTargetEntry.value = catalogById.value.get(plugin.id)
  updateTargetId.value = plugin.id
  updateTargetVersion.value = version ?? updateById.value.get(plugin.id)?.latest_version
  updateDetailOpen.value = false
  updateModalOpen.value = true
}

function openUpdateDetails(plugin: PluginInfo) {
  const entry = catalogEntryFor(plugin)
  if (!entry) {
    openUpdate(plugin)
    return
  }
  updateTargetEntry.value = entry
  updateTargetId.value = plugin.id
  updateTargetVersion.value = undefined
  drawerOpen.value = false
  updateDetailOpen.value = true
}

function onDetailInstall(entry: CatalogEntry, version?: string) {
  updateTargetEntry.value = entry
  updateTargetId.value = entry.id
  updateTargetVersion.value = version
  updateDetailOpen.value = false
  updateModalOpen.value = true
}

async function onPluginUpdated(info: PluginInfo) {
  // The updated package may ship a new frontend bundle, so drop the old one first.
  await pluginLoader.unload(info.id, { forget: true })
  await inventory.reload(true)
  await pluginLoader.unloadDisabled(inventory.plugins.value)
  await Promise.all([
    inventory.reloadUpdates(),
    inventory.reloadCatalog(),
    pluginLoader.loadNew(),
  ])
}

async function refreshAll() {
  await Promise.all([
    inventory.reload(),
    inventory.reloadUpdates(),
    inventory.reloadCatalog(),
  ])
}

function openDrawer(plugin: PluginInfo, tab: PluginDrawerTab) {
  selectedId.value = plugin.id
  drawerTab.value = tab
  drawerOpen.value = true
}

// The open drawer lives in the URL, so a link can point straight at one plugin.
watch(drawerOpen, open => {
  const { plugin: _plugin, ...rest } = route.query
  router.replace({
    query: open
      ? { ...rest, plugin: selectedId.value }
      : rest,
  })
})

// A link may point straight at one plugin. It is honoured once the list is
// there, and again whenever the query changes while the page stays mounted.
watch([
  plugins,
  () => route.query.plugin,
], ([list, id]) => {
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

async function enablePlugin(plugin: PluginInfo, approvePermissions?: boolean, replaceConflicts?: boolean) {
  togglingId.value = plugin.id
  const replaced = replaceConflicts
    ? (plugin.conflicts_enabled ?? [])
    : []
  try {
    await pluginApi.enable(plugin.id, approvePermissions, replaceConflicts)
    message.success($gettext('Plugin enabled'))
    for (const id of replaced)
      await pluginLoader.unload(id)
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
    await pluginLoader.unload(plugin.id)
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

/**
 * Turning a plugin off is immediate unless something on this node depends on
 * it, in which case the user sees what stops working first.
 */
async function requestDisable(plugin: PluginInfo) {
  togglingId.value = plugin.id
  let usage: PluginUsage | undefined
  try {
    usage = await pluginApi.getUsage(plugin.id)
  }
  catch {
    // An older node has no usage report, it is disabled as before.
  }
  finally {
    togglingId.value = ''
  }

  if (!usage || usage.total === 0) {
    await disablePlugin(plugin)
    return
  }

  const secondary = {
    style: { margin: 0, color: 'var(--ant-color-text-secondary)' },
  }
  modal.confirm({
    title: $gettext('Disable %{name}?', {
      name: localizedPluginName(plugin, gettext.current),
    }),
    content: h('div', {
      style: { display: 'flex', flexDirection: 'column', gap: '8px' },
    }, [
      h('p', {
        style: { margin: 0 },
      }, certificateUsageText(usage.total)),
      h('p', secondary, formatUsagePreview(previewUsage(usage))),
      h('p', secondary, $gettext('Renewal resumes when you turn it back on.')),
    ]),
    okText: $gettext('Disable'),
    okButtonProps: { danger: true },
    cancelText: $gettext('Cancel'),
    onOk: () => disablePlugin(plugin),
  })
}

function startEnable(plugin: PluginInfo, replaceConflicts: boolean) {
  // Enabling a plugin whose permissions are not approved yet has to go through
  // the approval dialog first.
  if (plugin.status === 'needs_approval') {
    pendingApproval.value = plugin
    pendingReplace.value = replaceConflicts
    approvalOpen.value = true
    return
  }

  void enablePlugin(plugin, undefined, replaceConflicts)
}

function toggle(plugin: PluginInfo, checked: boolean) {
  if (!checked) {
    void requestDisable(plugin)
    return
  }

  // The plugins that cannot be on together with this one are named first.
  const others = enabledConflictNames(plugin, plugins.value)
  if (others.length > 0) {
    const name = localizedPluginName(plugin, gettext.current)
    modal.confirm({
      title: $gettext('Enable %{name}?', { name }),
      content: enableReplacesText(name, others),
      okText: $gettext('Enable'),
      cancelText: $gettext('Cancel'),
      onOk: () => startEnable(plugin, true),
    })
    return
  }

  startEnable(plugin, false)
}

async function approvePermissions() {
  const plugin = pendingApproval.value
  if (!plugin)
    return

  approving.value = true
  try {
    await enablePlugin(plugin, true, pendingReplace.value)
    approvalOpen.value = false
    pendingApproval.value = undefined
    pendingReplace.value = false
  }
  finally {
    approving.value = false
  }
}

function confirmUninstall(plugin: PluginInfo) {
  modal.confirm({
    title: $gettext('Uninstall %{name}?', {
      name: localizedPluginName(plugin, gettext.current),
    }),
    content: $gettext('Its files, data and settings are removed from this node. Other nodes are not touched.'),
    okText: $gettext('Uninstall'),
    okButtonProps: { danger: true },
    cancelText: $gettext('Cancel'),
    async onOk() {
      try {
        await pluginApi.uninstall(plugin.id)
        message.success($gettext('Plugin uninstalled'))
        drawerOpen.value = false
        await pluginLoader.unload(plugin.id, { forget: true })
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

      <AButton class="installed-refresh" :loading="loading" @click="refreshAll">
        <template #icon>
          <ReloadOutlined />
        </template>
        {{ $gettext('Refresh') }}
      </AButton>

      <div class="installed-filter">
        <ASegmented v-model:value="filter" :options="filterOptions">
          <template #labelRender="option">
            <span class="filter-label">
              {{ option.label }}
              <span
                class="filter-count"
                :class="{
                  'is-update': option.value === 'updates' && counts.updates > 0,
                  'is-attention': option.value === 'attention' && counts.attention > 0,
                }"
              >{{ counts[option.value as InstalledFilter] }}</span>
            </span>
          </template>
        </ASegmented>
      </div>
    </div>

    <div v-if="counts.updates > 0 && filter !== 'updates'" class="updates-banner">
      <span class="updates-banner-icon">
        <ArrowUpOutlined />
      </span>
      <div class="updates-banner-body">
        <span class="updates-banner-text">
          {{ $ngettext('%{count} installed plugin has a newer release.', '%{count} installed plugins have a newer release.', counts.updates, { count: String(counts.updates) }) }}
        </span>
        <span v-if="singleUpdate" class="updates-banner-meta">
          <span>{{ localizedPluginName(singleUpdate.plugin, gettext.current) }}</span>
          <span class="updates-banner-version">v{{ singleUpdate.plugin.version }} → v{{ singleUpdate.update.latest_version }}</span>
        </span>
      </div>
      <div class="updates-banner-actions">
        <template v-if="singleUpdate">
          <AButton size="small" @click="openUpdateDetails(singleUpdate.plugin)">
            {{ $gettext('Release notes') }}
          </AButton>
          <AButton size="small" type="primary" @click="openUpdate(singleUpdate.plugin)">
            <template #icon>
              <ArrowUpOutlined />
            </template>
            {{ $gettext('Update') }}
          </AButton>
        </template>
        <AButton v-else size="small" type="primary" @click="filter = 'updates'">
          {{ $gettext('Show updates') }}
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
          :update="updateById.get(plugin.id)"
          :withdrawn="withdrawnIds.has(plugin.id)"
          :delisted="delistedById.get(plugin.id)"
          :toggling="togglingId === plugin.id"
          @open="tab => openDrawer(plugin, tab)"
          @toggle="checked => toggle(plugin, checked)"
          @sync="openSync(plugin)"
          @uninstall="confirmUninstall(plugin)"
          @update="openUpdate(plugin)"
          @update-details="openUpdateDetails(plugin)"
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
      :update="selected ? updateById.get(selected.id) : undefined"
      :has-nodes="hasNodes"
      :toggling="Boolean(selected) && togglingId === selected?.id"
      @toggle="checked => selected && toggle(selected, checked)"
      @sync="syncOpen = true"
      @uninstall="selected && confirmUninstall(selected)"
      @update="selected && openUpdate(selected)"
      @update-details="selected && openUpdateDetails(selected)"
      @updated="inventory.reload(true)"
    />
    <PluginDetailDrawer
      v-model:open="updateDetailOpen"
      :entry="updateTargetEntry"
      @install="onDetailInstall"
    />
    <InstallConfirmModal
      v-model:open="updateModalOpen"
      :entry="updateTargetEntry"
      :plugin-id="updateTargetId"
      :version="updateTargetVersion"
      @installed="onPluginUpdated"
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

// On a phone the search and refresh share the first row and the filter
// scrolls below them. Wider screens put the filter between the two.
.installed-search {
  flex: 1 1 0;
  min-width: 0;

  @media (min-width: 768px) {
    flex: 0 1 280px;
  }
}

.installed-refresh {
  flex: none;

  @media (min-width: 768px) {
    order: 3;
    margin-left: auto;
  }
}

// A phone cannot fit every filter, so the control scrolls instead of wrapping.
.installed-filter {
  flex: 1 1 100%;
  min-width: 0;
  overflow-x: auto;
  white-space: nowrap;
  scrollbar-width: none;

  @media (min-width: 768px) {
    flex: 0 1 auto;
    order: 2;
  }
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

  &.is-update {
    font-weight: 600;
    color: var(--ant-color-text-light-solid, #fff);
    background: var(--ant-color-primary);
  }

  &.is-attention {
    font-weight: 600;
    color: var(--ant-color-warning);
    background: var(--ant-color-warning-bg);
  }
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
