<script setup lang="tsx">
import type { CustomRenderArgs, StdTableColumn } from '@uozi-admin/curd'
import type { NginxLogData } from '@/api/nginx_log'
import type { TabOption } from '@/components/TabFilter'
import type { NginxLogRow } from '@/plugin/types'
import { CheckCircleOutlined, ExclamationCircleOutlined } from '@antdv-next/icons'
import { StdCurd } from '@uozi-admin/curd'
import { useRouteQuery } from '@vueuse/router'
import { Tag } from 'antdv-next'
import nginxLog from '@/api/nginx_log'
import PluginSlot from '@/components/PluginSlot'
import PluginSlotItem from '@/components/PluginSlot/PluginSlotItem.vue'
import { TabFilter } from '@/components/TabFilter'
import { NGINX_LOG_COLUMN_SLOT_PREFIX } from '@/plugin/slots'
import { usePluginStore } from '@/plugin/store'
import LegacyIndexingNotice from './LegacyIndexingNotice.vue'
import { applyPluginColumns, pluginColumnKey, stripPluginParams } from './pluginColumns'

const router = useRouter()

// Tab filter for log types
const activeLogType = useRouteQuery('type', 'access')

const tabOptions: TabOption[] = [
  {
    key: 'access',
    label: $gettext('Access Logs'),
    icon: h(CheckCircleOutlined),
    color: '#52c41a',
  },
  {
    key: 'error',
    label: $gettext('Error Logs'),
    icon: h(ExclamationCircleOutlined),
    color: '#ff4d4f',
  },
]

// Base columns that are always visible
const baseColumns: StdTableColumn[] = [
  {
    title: () => $gettext('Type'),
    dataIndex: 'type',
    customRender: (args: CustomRenderArgs) => {
      return args.record?.type === 'access' ? <Tag color="green">{ $gettext('Access Log') }</Tag> : <Tag color="orange">{ $gettext('Error Log') }</Tag>
    },
    sorter: true,
    width: 120,
  },
  {
    title: () => $gettext('Name'),
    dataIndex: 'name',
    sorter: true,
    search: {
      type: 'input',
    },
    width: 200,
  },
  {
    title: () => $gettext('Path'),
    dataIndex: 'path',
    sorter: true,
    search: {
      type: 'input',
    },
    ellipsis: true,
  },
]

// Actions column
const actionsColumn: StdTableColumn = {
  title: () => $gettext('Actions'),
  dataIndex: 'actions',
  fixed: 'right',
  width: 250,
}

// Columns plugins add to the list, one per registration. A registration whose
// condition rejects the list being shown adds no column at all.
const pluginStore = usePluginStore()

const pluginColumnSlots = computed(() => pluginStore.slotsByPrefix(NGINX_LOG_COLUMN_SLOT_PREFIX, { type: activeLogType.value }).map(item => ({
  ...item,
  columnKey: pluginColumnKey(`${item.registration.pluginId}:${item.key}`),
})))

const pluginColumnRules = computed(() => pluginColumnSlots.value.map(item => ({
  columnKey: item.columnKey,
  sortValue: item.registration.sortValue,
  filters: item.registration.filters,
})))

const pluginColumns = computed<StdTableColumn[]>(() => pluginColumnSlots.value.map(item => {
  const { registration } = item
  const hasFilters = (registration.filters?.length ?? 0) > 0

  return {
    title: () => $gettext(registration.label ?? item.key),
    dataIndex: item.columnKey,
    key: item.columnKey,
    sorter: !!registration.sortValue,
    filters: hasFilters
      ? registration.filters?.map(filter => ({ text: $gettext(filter.label), value: filter.value }))
      : undefined,
    customRender: (args: CustomRenderArgs) => {
      const row = args.record
      if (!row)
        return null

      return <PluginSlotItem registration={registration} context={{ row }} />
    },
    width: 160,
  }
}))

const columns = computed(() => [...baseColumns, ...pluginColumns.value, actionsColumn])

// The list arrives whole, so sorting and filtering by a plugin column happens
// here in the browser and the server only sees the parameters it knows.
const curdApi = {
  ...nginxLog,
  async getList(params: Record<string, unknown> = {}, config?: Record<string, unknown>) {
    const rules = pluginColumnRules.value
    const response = await nginxLog.getList(stripPluginParams(params), config)
    if (rules.length === 0)
      return response

    return { ...response, data: applyPluginColumns((response.data ?? []) as NginxLogRow[], params, rules) }
  },
}

function viewLog(record: NginxLogData) {
  router.push({
    path: `/nginx_log/${record.type}`,
    query: {
      path: record.path,
    },
  })
}
</script>

<template>
  <div>
    <LegacyIndexingNotice />

    <StdCurd
      :title="$gettext('Log List')"
      :columns="columns"
      :api="curdApi"
      disable-add
      disable-export
      disable-delete
      disable-trash
      disable-view
      disable-edit
      :overwrite-params="{
        type: activeLogType,
      }"
    >
      <template #beforeSearch>
        <TabFilter
          v-model:active-key="activeLogType"
          :options="tabOptions"
          size="middle"
        />
      </template>

      <template #beforeListActions>
        <div class="flex items-center gap-4">
          <PluginSlot name="nginx_log.list.toolbar" :context="{ type: activeLogType }" />
        </div>
      </template>
      <template #beforeActions="{ record }">
        <AButton type="link" size="small" @click="viewLog(record)">
          {{ $gettext('View') }}
        </AButton>

        <PluginSlot name="nginx_log.list.row.actions" :context="{ row: record }" />
      </template>
    </StdCurd>
  </div>
</template>
