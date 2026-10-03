<script setup lang="tsx">
import type { CustomRenderArgs, StdTableColumn } from '@uozi-admin/curd'
import type { UpstreamDiscovery } from '@/api/upstream_discovery'
import { StdCurd } from '@uozi-admin/curd'
import { Tag } from 'antdv-next'
import upstreamDiscovery, { refreshUpstreamDiscovery } from '@/api/upstream_discovery'
import GeneratedInclude, { formatInterval, RefreshResult } from '@/components/GeneratedInclude'
import { discoveryProviderLabel, loadDiscoveryProviders } from '../discovery'
import UpstreamDiscoveryEditor from './UpstreamDiscoveryEditor.vue'

// Upstreams whose servers an upstream.discovery plugin resolves.
const { message } = useGlobalApp()
const refreshingStates = ref<Record<number, boolean>>({})
const curdRef = useTemplateRef<{ refresh: () => void }>('curd')

onMounted(() => {
  loadDiscoveryProviders(true)
})

async function handleRefresh(record: UpstreamDiscovery) {
  if (!record.id)
    return
  refreshingStates.value[record.id] = true
  try {
    const refreshed = await refreshUpstreamDiscovery(record.id)
    if (refreshed.last_status === 'ok')
      message.success(refreshed.last_message || $gettext('Refreshed'))
    else
      message.error(refreshed.last_message || $gettext('Refresh failed'))
    curdRef.value?.refresh()
  }
  catch {
    // The request layer shows the error.
  }
  finally {
    refreshingStates.value[record.id] = false
  }
}

const columns: StdTableColumn[] = [
  {
    title: () => $gettext('Upstream'),
    dataIndex: 'upstream_name',
    sorter: true,
    pure: true,
    search: true,
    edit: {
      type: 'input',
      formItem: {
        required: true,
        extra: $gettext('The name nginx knows the upstream by: letters, digits, dots, hyphens and underscores.'),
      },
    },
  },
  {
    title: () => $gettext('Provider'),
    dataIndex: 'kind',
    customRender: ({ text }: CustomRenderArgs) => discoveryProviderLabel(text as string),
    edit: {
      type: (context: { formData: UpstreamDiscovery }) => {
        if (context.formData.enabled === undefined)
          context.formData.enabled = true
        return <UpstreamDiscoveryEditor v-model={context.formData} />
      },
      formItem: {
        hiddenLabelInEdit: true,
      },
    },
    pure: true,
  },
  {
    title: () => $gettext('Service'),
    dataIndex: 'service',
    hiddenInEdit: true,
    // The search form takes its input type from `edit`, which this column
    // does not have, so the type is spelled out.
    search: { type: 'input' },
    pure: true,
  },
  {
    title: () => $gettext('Configuration'),
    dataIndex: 'config',
    hiddenInTable: true,
    hiddenInEdit: true,
  },
  {
    title: () => $gettext('Refresh Interval'),
    dataIndex: 'refresh_seconds',
    customRender: ({ text }: CustomRenderArgs) => formatInterval(text as number),
    hiddenInEdit: true,
    pure: true,
  },
  {
    title: () => $gettext('Servers'),
    dataIndex: 'target_count',
    hiddenInEdit: true,
    pure: true,
  },
  {
    title: () => $gettext('Last Result'),
    dataIndex: 'last_status',
    customRender: ({ record }: CustomRenderArgs) => {
      const binding = record as UpstreamDiscovery
      return <RefreshResult status={binding.last_status} message={binding.last_message} lastRunAt={binding.last_run_at} />
    },
    hiddenInEdit: true,
    pure: true,
  },
  {
    title: () => $gettext('Include'),
    dataIndex: 'include',
    customRender: ({ record }: CustomRenderArgs) => {
      const binding = record as UpstreamDiscovery
      return <GeneratedInclude include={binding.include} path={binding.path} />
    },
    hiddenInEdit: true,
    pure: true,
  },
  {
    title: () => $gettext('Status'),
    dataIndex: 'enabled',
    customRender: ({ text }: CustomRenderArgs) => {
      return text
        ? <Tag color="green">{$gettext('Enabled')}</Tag>
        : <Tag color="red">{$gettext('Disabled')}</Tag>
    },
    edit: {
      type: 'switch',
    },
    sorter: true,
    pure: true,
  },
  {
    title: () => $gettext('Actions'),
    dataIndex: 'actions',
    fixed: 'right',
  },
]
</script>

<template>
  <StdCurd
    ref="curd"
    :title="$gettext('Service Discovery')"
    :columns="columns"
    :api="upstreamDiscovery"
    disable-export
  >
    <template #beforeActions="{ record }">
      <AButton
        type="link"
        size="small"
        :loading="refreshingStates[record.id] || false"
        @click="handleRefresh(record as UpstreamDiscovery)"
      >
        {{ $gettext('Refresh Now') }}
      </AButton>
    </template>
  </StdCurd>
</template>
