<script setup lang="ts">
import type { PluginMatrix, PluginMatrixCell, PluginMatrixNode, PluginMatrixRow, PluginSyncState } from '@/api/plugin_sync'
import { CloudServerOutlined, MoreOutlined, ReloadOutlined } from '@antdv-next/icons'
import { localizedPluginName } from '@/api/plugin'
import { getMatrix, syncPlugin } from '@/api/plugin_sync'
import gettext from '@/gettext'
import { getErrorMessage } from '@/lib/http'
import { usePluginInventory } from './inventory'

const { message } = useGlobalApp()
const inventory = usePluginInventory()

const loading = ref(false)
const syncingKey = ref('')
const matrix = ref<PluginMatrix>({ nodes: [], rows: [] })
const selectedNodeIds = ref<number[]>([])

type StateTone = 'success' | 'warning' | 'error' | 'info' | 'muted'

interface StatePreset {
  tone: StateTone
  label: () => string
}

const statePresets: Record<PluginSyncState, StatePreset> = {
  in_sync: { tone: 'success', label: () => $gettext('In sync') },
  outdated: { tone: 'warning', label: () => $gettext('Outdated') },
  missing: { tone: 'muted', label: () => $gettext('Not installed') },
  unsupported_platform: { tone: 'info', label: () => $gettext('Unsupported platform') },
  unsupported: { tone: 'info', label: () => $gettext('Unsupported') },
  offline: { tone: 'error', label: () => $gettext('Offline') },
  opted_out: { tone: 'muted', label: () => $gettext('Opted out') },
  error: { tone: 'error', label: () => $gettext('Error') },
}

/** Legend order: the states worth acting on come first. */
const legend = computed(() => (['in_sync', 'outdated', 'missing', 'offline', 'error', 'unsupported_platform', 'opted_out'] as PluginSyncState[])
  .map(state => ({ state, ...statePresets[state] })))

function stateOf(cell: PluginMatrixCell): StatePreset {
  return statePresets[cell.state] ?? { tone: 'muted', label: () => cell.state }
}

const nodeOptions = computed(() => matrix.value.nodes.map(node => ({
  value: node.id,
  label: node.name,
})))

const columns = computed(() => [
  { title: $gettext('Plugin'), dataIndex: 'plugin', width: 240, fixed: 'left' as const },
  ...matrix.value.nodes.map(node => ({
    title: node.name,
    dataIndex: `node:${node.id}`,
    width: 200,
  })),
  { title: $gettext('Action'), dataIndex: 'action', width: 150, fixed: 'right' as const },
])

const scrollX = computed(() => 390 + matrix.value.nodes.length * 200)

// The summary line tells at a glance whether anything on any node is behind.
const summary = computed(() => {
  const cells = matrix.value.rows.flatMap(row => row.cells)
  return {
    inSync: cells.filter(cell => cell.state === 'in_sync').length,
    pending: cells.filter(cell => cell.state === 'outdated' || cell.state === 'missing').length,
    failing: cells.filter(cell => cell.state === 'error' || cell.state === 'offline').length,
  }
})

// A row lists a plugin of this node, so its translations come from the
// installed list.
const installedById = computed(() => new Map(inventory.plugins.value.map(item => [item.id, item])))

function rowName(row: PluginMatrixRow) {
  const installed = installedById.value.get(row.plugin_id)
  return installed ? localizedPluginName(installed, gettext.current) : row.name
}

function nodeIdOf(dataIndex: string) {
  return Number(dataIndex.slice('node:'.length))
}

function cellOf(row: PluginMatrixRow, dataIndex: string): PluginMatrixCell | undefined {
  const nodeId = nodeIdOf(dataIndex)
  return row.cells.find(cell => cell.node_id === nodeId)
}

function nodeOf(dataIndex: string): PluginMatrixNode | undefined {
  const nodeId = nodeIdOf(dataIndex)
  return matrix.value.nodes.find(node => node.id === nodeId)
}

function platformOf(node: PluginMatrixNode) {
  return [node.os, node.arch].filter(Boolean).join('/')
}

async function load(showSpinner = true) {
  if (showSpinner)
    loading.value = true

  try {
    matrix.value = await getMatrix()
    // Drop selections pointing at nodes that are gone.
    const known = new Set(matrix.value.nodes.map(node => node.id))
    selectedNodeIds.value = selectedNodeIds.value.filter(id => known.has(id))
  }
  catch (error) {
    message.error(getErrorMessage(error, $gettext('Failed to load the plugin matrix')))
  }
  finally {
    loading.value = false
  }
}

async function sync(pluginId: string, nodeIds: number[], key: string) {
  syncingKey.value = key
  try {
    const { results } = await syncPlugin(pluginId, nodeIds)
    const failed = results.filter(result => !result.success)
    if (failed.length === 0)
      message.success($gettext('Plugin synchronized to %{count} node(s)', { count: String(results.length) }))
    else
      message.warning(failed.map(result => `${result.node}: ${result.error}`).join('\n'))

    await load(false)
  }
  catch (error) {
    message.error(getErrorMessage(error, $gettext('Failed to synchronize the plugin')))
  }
  finally {
    syncingKey.value = ''
  }
}

/** Every cell action ends up aligning that node with this controller. */
function cellActions(row: PluginMatrixRow, cell: PluginMatrixCell) {
  const items: { key: string, label: string }[] = []

  if (cell.state === 'missing')
    items.push({ key: 'install', label: $gettext('Install') })
  else if (cell.version && cell.version !== row.version)
    items.push({ key: 'update', label: $gettext('Update to %{version}', { version: row.version }) })

  if (cell.state !== 'missing' && cell.enabled !== row.enabled)
    items.push({ key: 'toggle', label: row.enabled ? $gettext('Enable') : $gettext('Disable') })

  items.push({ key: 'sync', label: $gettext('Sync to this node') })
  return items
}

function isCellActionable(cell: PluginMatrixCell) {
  return cell.state !== 'opted_out' && cell.state !== 'offline'
}

function cellKey(row: PluginMatrixRow, cell: PluginMatrixCell) {
  return `${row.plugin_id}:${cell.node_id}`
}

onMounted(() => load())
</script>

<template>
  <div>
    <div class="matrix-toolbar">
      <p class="mb-0 max-w-2xl text-gray-500">
        {{ $gettext('What every child node has installed, compared with this controller. Switching the selected node in the header manages that node directly.') }}
      </p>

      <div class="flex flex-wrap items-center gap-2">
        <ASelect
          v-if="matrix.nodes.length > 0"
          v-model:value="selectedNodeIds"
          mode="multiple"
          class="matrix-node-select"
          :options="nodeOptions"
          :max-tag-count="2"
          :placeholder="$gettext('All nodes')"
          allow-clear
        />
        <AButton :loading="loading" @click="load()">
          <template #icon>
            <ReloadOutlined />
          </template>
          {{ $gettext('Refresh') }}
        </AButton>
      </div>
    </div>

    <div v-if="matrix.nodes.length > 0" class="matrix-summary">
      <span class="matrix-summary-item">
        <span class="state-dot is-success" />
        {{ $gettext('%{count} in sync', { count: String(summary.inSync) }) }}
      </span>
      <span class="matrix-summary-item">
        <span class="state-dot is-warning" />
        {{ $gettext('%{count} pending', { count: String(summary.pending) }) }}
      </span>
      <span class="matrix-summary-item">
        <span class="state-dot is-error" />
        {{ $gettext('%{count} failing', { count: String(summary.failing) }) }}
      </span>
      <span class="matrix-legend">
        <span
          v-for="item in legend"
          :key="item.state"
          class="matrix-legend-item"
        >
          <span class="state-dot" :class="`is-${item.tone}`" />
          {{ item.label() }}
        </span>
      </span>
    </div>

    <div v-if="!loading && matrix.nodes.length === 0" class="matrix-empty">
      <span class="matrix-empty-icon">
        <CloudServerOutlined />
      </span>
      <h3 class="matrix-empty-title">
        {{ $gettext('This instance has no child node') }}
      </h3>
      <p class="matrix-empty-text">
        {{ $gettext('Add a node first, then plugins can be installed on it from here.') }}
      </p>
      <RouterLink to="/nodes">
        <AButton type="primary">
          {{ $gettext('Manage nodes') }}
        </AButton>
      </RouterLink>
    </div>

    <ATable
      v-else
      :columns="columns"
      :data-source="matrix.rows"
      :loading="loading"
      row-key="plugin_id"
      size="small"
      :pagination="false"
      :scroll="{ x: scrollX }"
    >
      <template #emptyText>
        <AEmpty :description="$gettext('No plugins installed')" />
      </template>

      <template #headerCell="{ column }">
        <div v-if="String(column.dataIndex).startsWith('node:') && nodeOf(String(column.dataIndex))" class="matrix-node-head">
          <span
            class="state-dot"
            :class="nodeOf(String(column.dataIndex))!.online ? 'is-success' : 'is-error'"
          />
          <span class="truncate">{{ nodeOf(String(column.dataIndex))!.name }}</span>
          <span v-if="platformOf(nodeOf(String(column.dataIndex))!)" class="matrix-node-platform">
            {{ platformOf(nodeOf(String(column.dataIndex))!) }}
          </span>
        </div>
      </template>

      <template #bodyCell="{ column, record }">
        <template v-if="column.dataIndex === 'plugin'">
          <div class="min-w-0">
            <div class="flex items-center gap-2">
              <span class="truncate font-medium">{{ rowName(record) }}</span>
              <span class="matrix-version">v{{ record.version }}</span>
            </div>
            <div class="matrix-plugin-meta">
              <span class="truncate">{{ record.plugin_id }}</span>
              <span class="flex-none">· {{ record.enabled ? $gettext('Enabled') : $gettext('Disabled') }}</span>
            </div>
          </div>
        </template>

        <template v-else-if="column.dataIndex === 'action'">
          <AButton
            type="link"
            size="small"
            class="px-0"
            :loading="syncingKey === record.plugin_id"
            @click="sync(record.plugin_id, selectedNodeIds, record.plugin_id)"
          >
            {{ selectedNodeIds.length > 0 ? $gettext('Sync to selected') : $gettext('Sync to all') }}
          </AButton>
        </template>

        <template v-else-if="String(column.dataIndex).startsWith('node:')">
          <div v-if="cellOf(record, String(column.dataIndex))" class="matrix-cell">
            <ATooltip :title="cellOf(record, String(column.dataIndex))!.message || undefined">
              <div class="min-w-0">
                <div class="matrix-cell-state">
                  <span class="state-dot" :class="`is-${stateOf(cellOf(record, String(column.dataIndex))!).tone}`" />
                  <span class="truncate">{{ stateOf(cellOf(record, String(column.dataIndex))!).label() }}</span>
                </div>
                <div class="matrix-cell-version">
                  {{ cellOf(record, String(column.dataIndex))!.version ? `v${cellOf(record, String(column.dataIndex))!.version}` : '-' }}
                </div>
              </div>
            </ATooltip>

            <ADropdown
              v-if="isCellActionable(cellOf(record, String(column.dataIndex))!)"
              :trigger="['click']"
              placement="bottomRight"
              :menu="{
                items: cellActions(record, cellOf(record, String(column.dataIndex))!),
                onClick: () => sync(
                  record.plugin_id,
                  [cellOf(record, String(column.dataIndex))!.node_id],
                  cellKey(record, cellOf(record, String(column.dataIndex))!),
                ),
              }"
            >
              <AButton
                type="text"
                size="small"
                class="matrix-cell-action"
                :loading="syncingKey === cellKey(record, cellOf(record, String(column.dataIndex))!)"
              >
                <template #icon>
                  <MoreOutlined />
                </template>
              </AButton>
            </ADropdown>
          </div>
          <span v-else class="text-gray-400">-</span>
        </template>
      </template>
    </ATable>
  </div>
</template>

<style lang="less" scoped>
.matrix-toolbar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 16px;
}

.matrix-node-select {
  min-width: 220px;
}

.matrix-summary {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px 16px;
  margin-bottom: 12px;
  font-size: 13px;
}

.matrix-summary-item,
.matrix-legend-item {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.matrix-legend {
  display: inline-flex;
  flex-wrap: wrap;
  gap: 6px 12px;
  margin-left: auto;
  font-size: 12px;
  color: var(--ant-color-text-tertiary);
}

.state-dot {
  flex: none;
  display: inline-block;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--ant-color-text-quaternary);

  &.is-success {
    background: var(--ant-color-success);
  }

  &.is-warning {
    background: var(--ant-color-warning);
  }

  &.is-error {
    background: var(--ant-color-error);
  }

  &.is-info {
    background: var(--ant-color-info);
  }
}

.matrix-node-head {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}

.matrix-node-platform {
  flex: none;
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 11px;
  font-weight: 400;
  color: var(--ant-color-text-quaternary);
}

.matrix-version {
  flex: none;
  padding: 0 6px;
  font-size: 11px;
  font-variant-numeric: tabular-nums;
  line-height: 16px;
  color: var(--ant-color-text-secondary);
  background: var(--ant-color-fill-tertiary);
  border-radius: 999px;
}

.matrix-plugin-meta {
  display: flex;
  gap: 4px;
  min-width: 0;
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 12px;
  color: var(--ant-color-text-quaternary);
}

.matrix-cell {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 4px;
  min-width: 0;
}

.matrix-cell-state {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}

.matrix-cell-version {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 12px;
  color: var(--ant-color-text-quaternary);
}

.matrix-cell-action {
  flex: none;
}

.matrix-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 48px 16px;
  text-align: center;
  border: 1px dashed var(--ant-color-border);
  border-radius: var(--ant-border-radius-lg);
}

.matrix-empty-icon {
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

.matrix-empty-title {
  margin: 0 0 6px;
  font-size: 16px;
  font-weight: 600;
}

.matrix-empty-text {
  max-width: 420px;
  margin: 0 0 20px;
  color: var(--ant-color-text-secondary);
}
</style>
