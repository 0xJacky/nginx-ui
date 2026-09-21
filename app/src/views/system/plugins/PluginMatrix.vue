<script setup lang="ts">
import type { PluginMatrix, PluginMatrixCell, PluginMatrixNode, PluginMatrixRow, PluginSyncState } from '@/api/plugin_sync'
import { CloudServerOutlined, MoreOutlined, ReloadOutlined } from '@antdv-next/icons'
import { getMatrix, syncPlugin } from '@/api/plugin_sync'
import { getErrorMessage } from '@/lib/http'

const { message } = App.useApp()

const loading = ref(false)
const syncingKey = ref('')
const matrix = ref<PluginMatrix>({ nodes: [], rows: [] })
const selectedNodeIds = ref<number[]>([])

interface StatePreset {
  color: string
  label: () => string
}

const statePresets: Record<PluginSyncState, StatePreset> = {
  in_sync: { color: 'green', label: () => $gettext('In sync') },
  outdated: { color: 'orange', label: () => $gettext('Outdated') },
  missing: { color: 'default', label: () => $gettext('Not installed') },
  unsupported_platform: { color: 'purple', label: () => $gettext('Unsupported platform') },
  unsupported: { color: 'purple', label: () => $gettext('Unsupported') },
  offline: { color: 'red', label: () => $gettext('Offline') },
  opted_out: { color: 'default', label: () => $gettext('Opted out') },
  error: { color: 'red', label: () => $gettext('Error') },
}

function stateOf(cell: PluginMatrixCell): StatePreset {
  return statePresets[cell.state] ?? { color: 'default', label: () => cell.state }
}

const nodeOptions = computed(() => matrix.value.nodes.map(node => ({
  value: node.id,
  label: node.name,
})))

const columns = computed(() => [
  { title: $gettext('Plugin'), dataIndex: 'plugin', width: 220, fixed: 'left' as const },
  ...matrix.value.nodes.map(node => ({
    title: node.name,
    dataIndex: `node:${node.id}`,
    width: 190,
  })),
  { title: $gettext('Action'), dataIndex: 'action', width: 140, fixed: 'right' as const },
])

const scrollX = computed(() => 360 + matrix.value.nodes.length * 190)

function cellOf(row: PluginMatrixRow, dataIndex: string): PluginMatrixCell | undefined {
  const nodeId = Number(dataIndex.slice('node:'.length))
  return row.cells.find(cell => cell.node_id === nodeId)
}

function nodeOf(dataIndex: string): PluginMatrixNode | undefined {
  const nodeId = Number(dataIndex.slice('node:'.length))
  return matrix.value.nodes.find(node => node.id === nodeId)
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

onMounted(() => load())
</script>

<template>
  <div>
    <div class="mb-4 flex flex-wrap items-center justify-between gap-2">
      <p class="mb-0 max-w-2xl text-gray-500">
        {{ $gettext('What every child node has installed, compared with this controller. Switching the selected node in the header manages that node directly.') }}
      </p>

      <ASpace wrap>
        <ASelect
          v-model:value="selectedNodeIds"
          mode="multiple"
          class="min-w-56"
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
      </ASpace>
    </div>

    <AEmpty v-if="!loading && matrix.nodes.length === 0">
      <template #image>
        <CloudServerOutlined class="text-5xl text-gray-400" />
      </template>
      <template #description>
        <div class="mx-auto max-w-md">
          <p class="mb-1 font-medium">
            {{ $gettext('This instance has no child node') }}
          </p>
          <p class="mb-0 text-gray-500">
            {{ $gettext('Add a node first, then plugins can be installed on it from here.') }}
          </p>
        </div>
      </template>
    </AEmpty>

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

      <template #bodyCell="{ column, record }">
        <template v-if="column.dataIndex === 'plugin'">
          <div class="min-w-0">
            <div class="font-medium">
              {{ record.name }}
            </div>
            <div class="truncate font-mono text-xs text-gray-400">
              {{ record.plugin_id }} · {{ record.version }}
            </div>
          </div>
        </template>

        <template v-else-if="column.dataIndex === 'action'">
          <AButton
            type="link"
            size="small"
            :loading="syncingKey === record.plugin_id"
            @click="sync(record.plugin_id, selectedNodeIds, record.plugin_id)"
          >
            {{ selectedNodeIds.length > 0 ? $gettext('Sync to selected') : $gettext('Sync to all') }}
          </AButton>
        </template>

        <template v-else-if="String(column.dataIndex).startsWith('node:')">
          <div v-if="cellOf(record, String(column.dataIndex))" class="flex items-center justify-between gap-1">
            <div class="min-w-0">
              <ATooltip :title="cellOf(record, String(column.dataIndex))!.message || undefined">
                <ATag :color="stateOf(cellOf(record, String(column.dataIndex))!).color">
                  {{ stateOf(cellOf(record, String(column.dataIndex))!).label() }}
                </ATag>
              </ATooltip>
              <div class="truncate font-mono text-xs text-gray-400">
                {{ cellOf(record, String(column.dataIndex))!.version || '-' }}
              </div>
            </div>

            <ADropdown
              v-if="isCellActionable(cellOf(record, String(column.dataIndex))!)"
              :trigger="['click']"
              placement="bottomRight"
            >
              <AButton
                type="text"
                size="small"
                :loading="syncingKey === `${record.plugin_id}:${nodeOf(String(column.dataIndex))?.id}`"
              >
                <MoreOutlined />
              </AButton>
              <template #popupRender>
                <AMenu
                  :items="cellActions(record, cellOf(record, String(column.dataIndex))!).map(item => ({
                    key: item.key,
                    label: item.label,
                    onClick: () => sync(
                      record.plugin_id,
                      [cellOf(record, String(column.dataIndex))!.node_id],
                      `${record.plugin_id}:${cellOf(record, String(column.dataIndex))!.node_id}`,
                    ),
                  }))"
                />
              </template>
            </ADropdown>
          </div>
          <span v-else class="text-gray-400">-</span>
        </template>
      </template>
    </ATable>
  </div>
</template>

<style lang="less" scoped>
</style>
