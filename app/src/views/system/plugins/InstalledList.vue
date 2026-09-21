<script setup lang="ts">
import type { BadgeProps } from 'antdv-next'
import type { PluginInfo, PluginStatus } from '@/api/plugin'
import type { PluginNodeResult } from '@/api/plugin_sync'
import { AppstoreOutlined, ExperimentOutlined, PlusOutlined, ReloadOutlined } from '@antdv-next/icons'
import { useIntervalFn } from '@vueuse/core'
import nodeApi from '@/api/node'
import pluginApi from '@/api/plugin'
import { syncPlugin } from '@/api/plugin_sync'
import NodeSelector from '@/components/NodeSelector'
import { getErrorMessage } from '@/lib/http'
import { usePluginLoader, usePluginStore } from '@/plugin'
import InstallModal from './InstallModal.vue'
import LogsDrawer from './LogsDrawer.vue'
import PermissionApprovalModal from './PermissionApprovalModal.vue'
import SettingsDrawer from './SettingsDrawer.vue'
import SyncPolicyEditor from './SyncPolicyEditor.vue'

const { message } = App.useApp()
const pluginStore = usePluginStore()

const loading = ref(false)
const plugins = ref<PluginInfo[]>([])
const togglingId = ref('')
const approving = ref(false)

const installOpen = ref(false)
const settingsOpen = ref(false)
const logsOpen = ref(false)
const approvalOpen = ref(false)
const devUrlOpen = ref(false)

const selected = ref<PluginInfo>()
const pendingApproval = ref<PluginInfo>()
const devUrlDraft = ref('')

// The cluster columns only make sense once this instance has a child node.
const hasNodes = ref(false)

const columns = computed(() => [
  { title: $gettext('Plugin'), dataIndex: 'name' },
  { title: $gettext('Version'), dataIndex: 'version', width: 110 },
  { title: $gettext('Capabilities'), dataIndex: 'capabilities' },
  { title: $gettext('Status'), dataIndex: 'status', width: 150 },
  { title: $gettext('Enabled'), dataIndex: 'enabled', width: 100 },
  ...(hasNodes.value
    ? [{ title: $gettext('Auto install to nodes'), dataIndex: 'sync_policy', width: 220 }]
    : []),
  { title: $gettext('Action'), dataIndex: 'action', width: hasNodes.value ? 300 : 220 },
])

const syncOpen = ref(false)
const syncing = ref(false)
const syncTarget = ref<PluginInfo>()
const syncNodeIds = ref<number[]>([])
const syncResults = ref<PluginNodeResult[]>([])

async function loadNodes() {
  try {
    const { data } = await nodeApi.getList({ enabled: true })
    hasNodes.value = data.length > 0
  }
  catch {
    hasNodes.value = false
  }
}

function openSync(record: PluginInfo) {
  syncTarget.value = record
  syncNodeIds.value = [...(record.sync_node_ids ?? [])]
  syncResults.value = []
  syncOpen.value = true
}

async function runSync() {
  const record = syncTarget.value
  if (!record)
    return

  syncing.value = true
  try {
    const { results } = await syncPlugin(record.id, syncNodeIds.value)
    syncResults.value = results
    if (results.every(result => result.success))
      message.success($gettext('Plugin synchronized to %{count} node(s)', { count: String(results.length) }))
    else
      message.warning($gettext('Some nodes could not be synchronized'))
  }
  catch (error) {
    message.error(getErrorMessage(error, $gettext('Failed to synchronize the plugin')))
  }
  finally {
    syncing.value = false
  }
}

interface StatusPreset {
  badge: BadgeProps['status']
  label: () => string
}

const statusPresets: Record<PluginStatus, StatusPreset> = {
  installed: { badge: 'default', label: () => $gettext('Installed') },
  starting: { badge: 'processing', label: () => $gettext('Starting') },
  running: { badge: 'success', label: () => $gettext('Running') },
  stopped: { badge: 'default', label: () => $gettext('Stopped') },
  error: { badge: 'error', label: () => $gettext('Error') },
  missing: { badge: 'error', label: () => $gettext('Missing') },
  incompatible: { badge: 'error', label: () => $gettext('Incompatible') },
  needs_approval: { badge: 'warning', label: () => $gettext('Needs approval') },
}

function statusOf(record: PluginInfo): StatusPreset {
  return statusPresets[record.status] ?? { badge: 'default', label: () => record.status }
}

/** A plugin the host cannot run at all must not offer a toggle. */
function isToggleDisabled(record: PluginInfo) {
  return record.status === 'incompatible' || record.status === 'missing'
}

const pluginLoader = usePluginLoader()

// A freshly installed bundle is picked up without a page reload.
async function onInstalled() {
  await loadPlugins()
  await pluginLoader.loadNew()
}

async function loadPlugins(showSpinner = true) {
  if (showSpinner)
    loading.value = true

  try {
    plugins.value = await pluginApi.getList()
  }
  catch (error) {
    message.error(getErrorMessage(error, $gettext('Failed to load the plugin list')))
  }
  finally {
    loading.value = false
  }
}

// A plugin that is still starting settles within a few seconds, so poll until
// nothing is in flight instead of asking the user to refresh.
const hasTransientState = computed(() => plugins.value.some(item => item.status === 'starting'))

const { pause, resume } = useIntervalFn(() => loadPlugins(false), 5000, { immediate: false })

watch(hasTransientState, value => {
  if (value)
    resume()
  else
    pause()
})

async function enablePlugin(record: PluginInfo, approvePermissions?: boolean) {
  togglingId.value = record.id
  try {
    await pluginApi.enable(record.id, approvePermissions)
    message.success($gettext('Plugin enabled'))
    await loadPlugins(false)
    await pluginLoader.loadNew()
  }
  catch (error) {
    message.error(getErrorMessage(error, $gettext('Failed to enable the plugin')))
    await loadPlugins(false)
  }
  finally {
    togglingId.value = ''
  }
}

async function disablePlugin(record: PluginInfo) {
  togglingId.value = record.id
  try {
    await pluginApi.disable(record.id)
    message.success($gettext('Plugin disabled'))
    await loadPlugins(false)
  }
  catch (error) {
    message.error(getErrorMessage(error, $gettext('Failed to disable the plugin')))
    await loadPlugins(false)
  }
  finally {
    togglingId.value = ''
  }
}

function toggle(record: PluginInfo, checked: boolean) {
  if (!checked) {
    disablePlugin(record)
    return
  }

  // Enabling a plugin whose permissions are not approved yet has to go through
  // the approval dialog first.
  if (record.status === 'needs_approval') {
    pendingApproval.value = record
    approvalOpen.value = true
    return
  }

  enablePlugin(record)
}

async function approvePermissions() {
  const record = pendingApproval.value
  if (!record)
    return

  approving.value = true
  try {
    await enablePlugin(record, true)
    approvalOpen.value = false
    pendingApproval.value = undefined
  }
  finally {
    approving.value = false
  }
}

async function uninstall(record: PluginInfo) {
  try {
    await pluginApi.uninstall(record.id)
    message.success($gettext('Plugin uninstalled'))
    await loadPlugins(false)
  }
  catch (error) {
    message.error(getErrorMessage(error, $gettext('Failed to uninstall the plugin')))
  }
}

function openSettings(record: PluginInfo) {
  selected.value = record
  settingsOpen.value = true
}

function openLogs(record: PluginInfo) {
  selected.value = record
  logsOpen.value = true
}

function openDevUrl(open: boolean) {
  devUrlOpen.value = open
  if (open)
    devUrlDraft.value = pluginStore.devPluginUrl
}

function saveDevUrl() {
  pluginStore.devPluginUrl = devUrlDraft.value.trim()
  devUrlOpen.value = false
  message.success($gettext('Reload the page to apply the development plugin URL'))
}

function clearDevUrl() {
  devUrlDraft.value = ''
  pluginStore.devPluginUrl = ''
  devUrlOpen.value = false
  message.success($gettext('Reload the page to apply the development plugin URL'))
}

onMounted(() => {
  loadPlugins()
  loadNodes()
})
onUnmounted(pause)
</script>

<template>
  <div>
    <div class="mb-4 flex flex-wrap items-center justify-between gap-2">
      <p class="mb-0 max-w-2xl text-gray-500">
        {{ $gettext('Plugins extend Nginx UI with extra capabilities. Only install bundles from authors you trust.') }}
      </p>

      <ASpace wrap>
        <APopover
          :open="devUrlOpen"
          trigger="click"
          placement="bottomRight"
          :title="$gettext('Development plugin URL')"
          @open-change="openDevUrl"
        >
          <template #content>
            <div class="dev-url-popover">
              <p class="mb-2 text-gray-500">
                {{ $gettext('Address of a plugin.json served by any static server. The plugin is loaded in addition to the installed ones.') }}
              </p>
              <AInput
                v-model:value="devUrlDraft"
                placeholder="http://localhost:5173/plugin.json"
                allow-clear
                @press-enter="saveDevUrl"
              />
              <div class="mt-2 flex justify-end gap-2">
                <AButton size="small" @click="clearDevUrl">
                  {{ $gettext('Clear') }}
                </AButton>
                <AButton size="small" type="primary" @click="saveDevUrl">
                  {{ $gettext('Save') }}
                </AButton>
              </div>
            </div>
          </template>
          <AButton :type="pluginStore.devPluginUrl ? 'primary' : 'default'" ghost>
            <template #icon>
              <ExperimentOutlined />
            </template>
            {{ $gettext('Dev plugin') }}
          </AButton>
        </APopover>

        <AButton :loading="loading" @click="loadPlugins()">
          <template #icon>
            <ReloadOutlined />
          </template>
          {{ $gettext('Refresh') }}
        </AButton>

        <AButton type="primary" @click="installOpen = true">
          <template #icon>
            <PlusOutlined />
          </template>
          {{ $gettext('Install') }}
        </AButton>
      </ASpace>
    </div>

    <ATable
      :columns="columns"
      :data-source="plugins"
      :loading="loading"
      row-key="id"
      size="small"
      :pagination="false"
      :scroll="{ x: 900 }"
    >
      <template #emptyText>
        <AEmpty :description="$gettext('No plugins installed')" />
      </template>
      <template #bodyCell="{ column, record }">
        <template v-if="column.dataIndex === 'name'">
          <div class="flex items-center gap-2">
            <img
              v-if="record.icon_url"
              :src="record.icon_url"
              class="plugin-icon"
              alt=""
            >
            <AppstoreOutlined v-else class="text-gray-400" />
            <div class="min-w-0">
              <div class="font-medium">
                {{ record.name }}
              </div>
              <div class="truncate font-mono text-xs text-gray-400">
                {{ record.id }}
              </div>
            </div>
          </div>
        </template>

        <template v-else-if="column.dataIndex === 'capabilities'">
          <div v-if="record.capabilities?.length" class="flex flex-wrap gap-1">
            <ATag v-for="capability in record.capabilities" :key="capability">
              {{ capability }}
            </ATag>
          </div>
          <span v-else class="text-gray-400">-</span>
        </template>

        <template v-else-if="column.dataIndex === 'status'">
          <ATooltip :title="record.last_error || undefined">
            <ABadge :status="statusOf(record).badge" :text="statusOf(record).label()" />
          </ATooltip>
        </template>

        <template v-else-if="column.dataIndex === 'enabled'">
          <ASwitch
            :checked="record.enabled"
            :disabled="isToggleDisabled(record)"
            :loading="togglingId === record.id"
            @change="checked => toggle(record, Boolean(checked))"
          />
        </template>

        <template v-else-if="column.dataIndex === 'sync_policy'">
          <SyncPolicyEditor :plugin="record" @updated="loadPlugins(false)" />
        </template>

        <template v-else-if="column.dataIndex === 'action'">
          <ASpace :size="0" wrap>
            <AButton type="link" size="small" @click="openSettings(record)">
              {{ $gettext('Settings') }}
            </AButton>
            <AButton type="link" size="small" @click="openLogs(record)">
              {{ $gettext('Logs') }}
            </AButton>
            <AButton
              v-if="hasNodes"
              type="link"
              size="small"
              @click="openSync(record)"
            >
              {{ $gettext('Sync to nodes') }}
            </AButton>
            <APopconfirm
              :title="$gettext('Uninstall %{name}? Its data and settings are removed.', { name: record.name })"
              :ok-text="$gettext('Uninstall')"
              :cancel-text="$gettext('Cancel')"
              @confirm="uninstall(record)"
            >
              <AButton type="link" size="small" danger>
                {{ $gettext('Uninstall') }}
              </AButton>
            </APopconfirm>
          </ASpace>
        </template>
      </template>
    </ATable>

    <AModal
      v-model:open="syncOpen"
      :title="$gettext('Sync %{name} to nodes', { name: syncTarget?.name ?? '' })"
      :width="640"
      :ok-text="$gettext('Sync')"
      :cancel-text="$gettext('Close')"
      :confirm-loading="syncing"
      @ok="runSync"
    >
      <p class="mb-2 text-gray-500">
        {{ $gettext('Leave every node unchecked to sync to all child nodes.') }}
      </p>
      <NodeSelector v-model:target="syncNodeIds" hidden-local />

      <div v-if="syncResults.length > 0" class="sync-results mt-4">
        <div
          v-for="result in syncResults"
          :key="result.node_id"
          class="sync-result-row"
        >
          <span class="font-medium">{{ result.node }}</span>
          <ATag v-if="result.success" color="green">
            {{ result.actions.length > 0 ? result.actions.join(', ') : $gettext('Already in sync') }}
          </ATag>
          <ATooltip v-else :title="result.error">
            <ATag color="red">
              {{ $gettext('Failed') }}
            </ATag>
          </ATooltip>
        </div>
      </div>
    </AModal>

    <InstallModal v-model:open="installOpen" @installed="onInstalled" />
    <SettingsDrawer v-model:open="settingsOpen" :plugin="selected" />
    <LogsDrawer v-model:open="logsOpen" :plugin="selected" />
    <PermissionApprovalModal
      v-model:open="approvalOpen"
      :plugin="pendingApproval"
      :confirm-loading="approving"
      @approve="approvePermissions"
    />
  </div>
</template>

<style lang="less" scoped>
.sync-results {
  border: 1px solid var(--ant-color-border-secondary);
  border-radius: var(--ant-border-radius);
  overflow: hidden;
}

.sync-result-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 6px 12px;

  & + & {
    border-top: 1px solid var(--ant-color-border-secondary);
  }
}

.plugin-icon {
  width: 20px;
  height: 20px;
  object-fit: contain;
  border-radius: 4px;
}

.dev-url-popover {
  width: 320px;
  max-width: 70vw;
}
</style>
