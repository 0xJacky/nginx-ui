<script setup lang="tsx">
import type { CustomRenderArgs, StdTableColumn } from '@uozi-admin/curd'
import type { BlocklistSource } from '@/api/blocklist'
import { StdCurd } from '@uozi-admin/curd'
import { Tag } from 'antdv-next'
import blocklistSource, { refreshBlocklistSource } from '@/api/blocklist'
import GeneratedInclude, { formatInterval, RefreshResult } from '@/components/GeneratedInclude'
import { blocklistKindLabel, loadBlocklistKinds } from './blocklists'
import BlocklistSourceEditor from './components/BlocklistSourceEditor.vue'

const { message } = useGlobalApp()
const refreshingStates = ref<Record<number, boolean>>({})
const curdRef = useTemplateRef<{ refresh: () => void }>('curd')

onMounted(() => {
  loadBlocklistKinds(true)
})

async function handleRefresh(record: BlocklistSource) {
  if (!record.id)
    return
  refreshingStates.value[record.id] = true
  try {
    const refreshed = await refreshBlocklistSource(record.id)
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
    title: () => $gettext('Name'),
    dataIndex: 'name',
    sorter: true,
    pure: true,
    search: true,
    edit: {
      type: 'input',
      formItem: {
        required: true,
      },
    },
  },
  {
    title: () => $gettext('Source Type'),
    dataIndex: 'kind',
    customRender: ({ text }: CustomRenderArgs) => blocklistKindLabel(text as string),
    edit: {
      type: (context: { formData: BlocklistSource }) => {
        if (context.formData.enabled === undefined)
          context.formData.enabled = true
        return <BlocklistSourceEditor v-model={context.formData} />
      },
      formItem: {
        hiddenLabelInEdit: true,
      },
    },
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
    title: () => $gettext('Entries'),
    dataIndex: 'entry_count',
    hiddenInEdit: true,
    pure: true,
  },
  {
    title: () => $gettext('Last Result'),
    dataIndex: 'last_status',
    customRender: ({ record }: CustomRenderArgs) => {
      const source = record as BlocklistSource
      return <RefreshResult status={source.last_status} message={source.last_message} lastRunAt={source.last_run_at} />
    },
    hiddenInEdit: true,
    pure: true,
  },
  {
    title: () => $gettext('Include'),
    dataIndex: 'include',
    customRender: ({ record }: CustomRenderArgs) => {
      const source = record as BlocklistSource
      return <GeneratedInclude include={source.include} path={source.path} />
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
    search: {
      type: 'select',
      select: {
        options: [
          { label: $gettext('Enabled'), value: 1 },
          { label: $gettext('Disabled'), value: 0 },
        ],
      },
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
  <div>
    <AAlert
      class="mb-4"
      type="info"
      show-icon
      :message="$gettext('Blocklist sources fetch lists of addresses to deny from plugins and write them as deny rules to a file of their own. Nothing is blocked until you add the include line of a source to a server or location block.')"
    />
    <StdCurd
      ref="curd"
      :title="$gettext('Blocklists')"
      :columns="columns"
      :api="blocklistSource"
      disable-export
    >
      <template #beforeActions="{ record }">
        <AButton
          type="link"
          size="small"
          :loading="refreshingStates[record.id] || false"
          @click="handleRefresh(record as BlocklistSource)"
        >
          {{ $gettext('Refresh Now') }}
        </AButton>
      </template>
    </StdCurd>
  </div>
</template>
