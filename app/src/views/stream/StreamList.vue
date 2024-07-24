<script setup lang="tsx">
import { Badge, message } from 'ant-design-vue'
import StdTable from '@/components/StdDesign/StdDataDisplay/StdTable.vue'
import type { customRender } from '@/components/StdDesign/StdDataDisplay/StdTableTransformer'
import { datetime } from '@/components/StdDesign/StdDataDisplay/StdTableTransformer'
import stream from '@/api/stream'
import { input } from '@/components/StdDesign/StdDataEntry'
import InspectConfig from '@/views/config/InspectConfig.vue'
import type { Column, JSXElements } from '@/components/StdDesign/types'
import StreamDuplicate from '@/views/stream/components/StreamDuplicate.vue'

const columns: Column[] = [{
  title: () => $gettext('Name'),
  dataIndex: 'name',
  sortable: true,
  pithy: true,
  edit: {
    type: input,
  },
  search: true,
}, {
  title: () => $gettext('Status'),
  dataIndex: 'enabled',
  customRender: (args: customRender) => {
    const template: JSXElements = []
    const { text } = args
    if (text === true || text > 0) {
      template.push(<Badge status="success"/>)
      template.push($gettext('Enabled'))
    }
    else {
      template.push(<Badge status="warning"/>)
      template.push($gettext('Disabled'))
    }

    return h('div', template)
  },
  sortable: true,
  pithy: true,
}, {
  title: () => $gettext('Updated at'),
  dataIndex: 'modified_at',
  customRender: datetime,
  sortable: true,
  pithy: true,
}, {
  title: () => $gettext('Action'),
  dataIndex: 'action',
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
  }).catch(e => {
    message.error(e?.message ?? $gettext('Server error'))
  })
}

const showDuplicator = ref(false)

const target = ref('')

function handle_click_duplicate(name: string) {
  showDuplicator.value = true
  target.value = name
}

const route = useRoute()

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
  }).catch(e => {
    message.error(e?.message ?? $gettext('Server error'))
  })
}
</script>

<template>
  <ACard :title="$gettext('Manage Streams')">
    <template #extra>
      <a @click="add">{{ $gettext('Add') }}</a>
    </template>

    <InspectConfig ref="inspect_config" />

    <StdTable
      ref="table"
      :api="stream"
      :columns="columns"
      row-key="name"
      disable-delete
      disable-view
      @click-edit="r => $router.push({
        path: `/stream/${r}`,
      })"
    >
      <template #actions="{ record }">
        <ADivider type="vertical" />
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
        <ADivider type="vertical" />
        <AButton
          type="link"
          size="small"
          @click="handle_click_duplicate(record.name)"
        >
          {{ $gettext('Duplicate') }}
        </AButton>
        <ADivider type="vertical" />
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
  </ACard>
</template>

<style scoped>

</style>
