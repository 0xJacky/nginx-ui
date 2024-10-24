<script setup lang="tsx">
import type { CustomRenderProps } from '@/components/StdDesign/StdDataDisplay/StdTableTransformer'
import type { Column, JSXElements } from '@/components/StdDesign/types'
import domain from '@/api/domain'
import StdTable from '@/components/StdDesign/StdDataDisplay/StdTable.vue'
import { datetime } from '@/components/StdDesign/StdDataDisplay/StdTableTransformer'
import { input, select } from '@/components/StdDesign/StdDataEntry'
import InspectConfig from '@/views/config/InspectConfig.vue'
import SiteDuplicate from '@/views/site/components/SiteDuplicate.vue'
import { Badge, message } from 'ant-design-vue'

const columns: Column[] = [{
  title: () => $gettext('Name'),
  dataIndex: 'name',
  sorter: true,
  pithy: true,
  edit: {
    type: input,
  },
  search: true,
}, {
  title: () => $gettext('Status'),
  dataIndex: 'enabled',
  customRender: (args: CustomRenderProps) => {
    const template: JSXElements = []
    const { text } = args
    if (text === true || text > 0) {
      template.push(<Badge status="success" />)
      template.push($gettext('Enabled'))
    }
    else {
      template.push(<Badge status="warning" />)
      template.push($gettext('Disabled'))
    }

    return h('div', template)
  },
  search: {
    type: select,
    mask: {
      true: $gettext('Enabled'),
      false: $gettext('Disabled'),
    },
  },
  sorter: true,
  pithy: true,
}, {
  title: () => $gettext('Updated at'),
  dataIndex: 'modified_at',
  customRender: datetime,
  sorter: true,
  pithy: true,
}, {
  title: () => $gettext('Action'),
  dataIndex: 'action',
}]

const table = ref()

const inspect_config = ref()

function enable(name: string) {
  domain.enable(name).then(() => {
    message.success($gettext('Enabled successfully'))
    table.value?.get_list()
    inspect_config.value?.test()
  }).catch(r => {
    message.error($gettext('Failed to enable %{msg}', { msg: r.message ?? '' }), 10)
  })
}

function disable(name: string) {
  domain.disable(name).then(() => {
    message.success($gettext('Disabled successfully'))
    table.value?.get_list()
    inspect_config.value?.test()
  }).catch(r => {
    message.error($gettext('Failed to disable %{msg}', { msg: r.message ?? '' }))
  })
}

function destroy(site_name: string) {
  domain.destroy(site_name).then(() => {
    table.value.get_list()
    message.success($gettext('Delete site: %{site_name}', { site_name }))
    inspect_config.value?.test()
  }).catch(e => {
    message.error(e?.message ?? $gettext('Server error'))
  })
}

const show_duplicator = ref(false)

const target = ref('')

function handle_click_duplicate(name: string) {
  show_duplicator.value = true
  target.value = name
}

const route = useRoute()

watch(route, () => {
  inspect_config.value?.test()
})
</script>

<template>
  <ACard :title="$gettext('Manage Sites')">
    <InspectConfig ref="inspect_config" />

    <StdTable
      ref="table"
      :api="domain"
      :columns="columns"
      row-key="name"
      disable-delete
      disable-view
      @click-edit="r => $router.push({
        path: `/sites/${r}`,
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
    <SiteDuplicate
      v-model:visible="show_duplicator"
      :name="target"
      @duplicated="() => table.get_list()"
    />
  </ACard>
</template>

<style scoped>

</style>
