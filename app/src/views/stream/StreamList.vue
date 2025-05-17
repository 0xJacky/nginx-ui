<script setup lang="tsx">
import type { EnvGroup } from '@/api/env_group'
import type { Stream } from '@/api/stream'
import type { CustomRender } from '@/components/StdDesign/StdDataDisplay/StdTableTransformer'
import type { Column, JSXElements } from '@/components/StdDesign/types'
import { Badge, message } from 'ant-design-vue'
import env_group from '@/api/env_group'
import stream from '@/api/stream'
import EnvGroupTabs from '@/components/EnvGroupTabs'
import StdBatchEdit from '@/components/StdDesign/StdDataDisplay/StdBatchEdit.vue'
import StdTable from '@/components/StdDesign/StdDataDisplay/StdTable.vue'
import { actualValueRender, datetime } from '@/components/StdDesign/StdDataDisplay/StdTableTransformer'
import { input, selector } from '@/components/StdDesign/StdDataEntry'
import { ConfigStatus } from '@/constants'
import InspectConfig from '@/views/config/InspectConfig.vue'
import envGroupColumns from '@/views/environments/group/columns'
import StreamDuplicate from '@/views/stream/components/StreamDuplicate.vue'

const columns: Column[] = [{
  title: () => $gettext('Name'),
  dataIndex: 'name',
  sorter: true,
  pithy: true,
  edit: {
    type: input,
  },
  search: true,
  width: 150,
}, {
  title: () => $gettext('Node Group'),
  dataIndex: 'env_group_id',
  customRender: actualValueRender('env_group.name'),
  edit: {
    type: selector,
    selector: {
      api: env_group,
      columns: envGroupColumns,
      recordValueIndex: 'name',
      selectionType: 'radio',
    },
  },
  sorter: true,
  pithy: true,
  batch: true,
  width: 150,
}, {
  title: () => $gettext('Status'),
  dataIndex: 'status',
  customRender: (args: CustomRender) => {
    const template: JSXElements = []
    const { text } = args
    if (text === ConfigStatus.Enabled) {
      template.push(<Badge status="success" />)
      template.push($gettext('Enabled'))
    }
    else if (text === ConfigStatus.Disabled) {
      template.push(<Badge status="warning" />)
      template.push($gettext('Disabled'))
    }

    return h('div', template)
  },
  sorter: true,
  pithy: true,
  width: 200,
}, {
  title: () => $gettext('Updated at'),
  dataIndex: 'modified_at',
  customRender: datetime,
  sorter: true,
  pithy: true,
  width: 200,
}, {
  title: () => $gettext('Action'),
  dataIndex: 'action',
  width: 250,
  fixed: 'right',
}]

const table = ref()

const inspect_config = ref()

function enable(name: string) {
  stream.enable(name).then(() => {
    message.success($gettext('Enabled successfully'))
    table.value?.get_list()
    inspect_config.value?.test()
  }).catch(r => {
    message.error($gettext('Failed to enable %{msg}', { msg: r.message ?? '' }), 10)
  })
}

function disable(name: string) {
  stream.disable(name).then(() => {
    message.success($gettext('Disabled successfully'))
    table.value?.get_list()
    inspect_config.value?.test()
  }).catch(r => {
    message.error($gettext('Failed to disable %{msg}', { msg: r.message ?? '' }))
  })
}

function destroy(stream_name: string) {
  stream.destroy(stream_name).then(() => {
    table.value.get_list()
    message.success($gettext('Delete stream: %{stream_name}', { stream_name }))
    inspect_config.value?.test()
  })
}

const showDuplicator = ref(false)

const target = ref('')

function handle_click_duplicate(name: string) {
  showDuplicator.value = true
  target.value = name
}

const route = useRoute()

const envGroupId = ref(Number.parseInt(route.query.env_group_id as string) || 0)
const envGroups = ref([]) as Ref<EnvGroup[]>

onMounted(async () => {
  let page = 1
  while (true) {
    try {
      const { data, pagination } = await env_group.get_list({ page })
      if (!data || !pagination)
        return
      envGroups.value.push(...data)
      if (data.length < pagination?.per_page) {
        return
      }
      page++
    }
    catch {
      return
    }
  }
})

watch(route, () => {
  inspect_config.value?.test()
})

const showAddStream = ref(false)
const name = ref('')
function add() {
  showAddStream.value = true
  name.value = ''
}

function handleAddStream() {
  stream.save(name.value, { name: name.value, content: 'server\t{\n\n}' }).then(() => {
    showAddStream.value = false
    table.value?.get_list()
    message.success($gettext('Added successfully'))
  })
}

const stdBatchEditRef = useTemplateRef('stdBatchEditRef')

async function handleClickBatchEdit(batchColumns: Column[], selectedRowKeys: string[], selectedRows: Stream[]) {
  stdBatchEditRef.value?.showModal(batchColumns, selectedRowKeys, selectedRows)
}

function handleBatchUpdated() {
  table.value?.get_list()
  table.value?.resetSelection()
}
</script>

<template>
  <ACard :title="$gettext('Manage Streams')">
    <template #extra>
      <div class="flex items-center cursor-default">
        <a class="mr-4" @click="add">{{ $gettext('Add') }}</a>
      </div>
    </template>

    <InspectConfig ref="inspect_config" />

    <EnvGroupTabs v-model:active-key="envGroupId" :env-groups="envGroups" />

    <StdTable
      ref="table"
      :api="stream"
      :columns="columns"
      row-key="name"
      disable-delete
      disable-view
      :scroll-x="800"
      :get-params="{
        env_group_id: envGroupId,
      }"
      @click-edit="r => $router.push({
        path: `/streams/${encodeURIComponent(r)}`,
      })"
      @click-batch-modify="handleClickBatchEdit"
    >
      <template #actions="{ record }">
        <AButton
          v-if="record.enabled"
          type="link"
          size="small"
          @click="disable(record.name)"
        >
          {{ $gettext('Disable') }}
        </AButton>
        <AButton
          v-else
          type="link"
          size="small"
          @click="enable(record.name)"
        >
          {{ $gettext('Enable') }}
        </AButton>
        <AButton
          type="link"
          size="small"
          @click="handle_click_duplicate(record.name)"
        >
          {{ $gettext('Duplicate') }}
        </AButton>
        <APopconfirm
          :cancel-text="$gettext('No')"
          :ok-text="$gettext('OK')"
          :title="$gettext('Are you sure you want to delete?')"
          :disabled="record.enabled"
          @confirm="destroy(record.name)"
        >
          <AButton
            type="link"
            size="small"
            :disabled="record.enabled"
          >
            {{ $gettext('Delete') }}
          </AButton>
        </APopconfirm>
      </template>
    </StdTable>
    <AModal
      v-model:open="showAddStream"
      :title="$gettext('Add Stream')"
      :mask="false"
      @ok="handleAddStream"
    >
      <AForm layout="vertical">
        <AFormItem :label="$gettext('Name')">
          <AInput v-model:value="name" />
        </AFormItem>
      </AForm>
    </AModal>
    <StreamDuplicate
      v-model:visible="showDuplicator"
      :name="target"
      @duplicated="() => table.get_list()"
    />
    <StdBatchEdit
      ref="stdBatchEditRef"
      :api="stream"
      :columns="columns"
      @save="handleBatchUpdated"
    />
  </ACard>
</template>

<style scoped>

</style>
