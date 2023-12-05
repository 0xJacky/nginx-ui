<script setup lang="tsx">
import { useGettext } from 'vue3-gettext'
import { Badge, Tag } from 'ant-design-vue'
import { h, provide } from 'vue'
import dayjs from 'dayjs'
import { input } from '@/components/StdDesign/StdDataEntry'
import type { customRender } from '@/components/StdDesign/StdDataDisplay/StdTableTransformer'
import { datetime } from '@/components/StdDesign/StdDataDisplay/StdTableTransformer'
import cert from '@/api/cert'
import type { Column } from '@/components/StdDesign/types'
import type { Cert } from '@/api/cert'
import { AutoCertState } from '@/constants'
import StdTable from '@/components/StdDesign/StdDataDisplay/StdTable.vue'

const { $gettext } = useGettext()

function notShowInAutoCert(record: Cert) {
  return record.auto_cert !== AutoCertState.Enable
}

const columns: Column[] = [{
  title: () => $gettext('Name'),
  dataIndex: 'name',
  sortable: true,
  pithy: true,
  customRender: (args: customRender) => {
    const { text, record } = args
    if (!text)
      return h('div', record.domain)

    return h('div', text)
  },
  edit: {
    type: input,
    show: notShowInAutoCert,
  },
  search: true,
}, {
  title: () => $gettext('Type'),
  dataIndex: 'auto_cert',
  customRender: (args: customRender) => {
    const template = []
    const { text } = args
    if (text === true || text > 0)
      template.push(<Tag bordered={false} color="processing">{$gettext('Managed Certificate')}</Tag>)

    else
      template.push(<Tag bordered={false} color="purple">{$gettext('General Certificate')}</Tag>)

    return h('div', template)
  },
  sortable: true,
  pithy: true,
}, {
  title: () => $gettext('SSL Certificate Path'),
  dataIndex: 'ssl_certificate_path',
  edit: {
    type: input,
    show: notShowInAutoCert,
  },
  hidden: true,
}, {
  title: () => $gettext('SSL Certificate Key Path'),
  dataIndex: 'ssl_certificate_key_path',
  edit: {
    type: input,
    show: notShowInAutoCert,
  },
  hidden: true,
}, {
  title: () => $gettext('Status'),
  dataIndex: 'certificate_info',
  customRender: (args: customRender) => {
    const template = []

    const text = args.text?.not_before && args.text?.not_after && !dayjs().isBefore(args.text?.not_before) && !dayjs().isAfter(args.text?.not_after)

    if (text) {
      template.push(<Badge status="success"/>)
      template.push($gettext('Valid'))
    }
    else {
      template.push(<Badge status="error"/>)
      template.push($gettext('Expired'))
    }

    return h('div', template)
  },
}, {
  title: () => $gettext('Not After'),
  dataIndex: ['certificate_info', 'not_after'],
  customRender: datetime,
  sortable: true,
  pithy: true,
}, {
  title: () => $gettext('Action'),
  dataIndex: 'action',
}]

// DO NOT REMOVE THESE LINES
const no_server_name = computed(() => {
  return false
})

provide('no_server_name', no_server_name)
</script>

<template>
  <ACard :title="$gettext('Certificates')">
    <template #extra>
      <a @click="$router.push('/certificates/add')">
        {{ $gettext('Add') }}
      </a>
    </template>
    <StdTable
      :api="cert"
      :columns="columns"
      @click-edit="id => $router.push(`/certificates/${id}`)"
    />
  </ACard>
</template>

<style lang="less" scoped>

</style>
