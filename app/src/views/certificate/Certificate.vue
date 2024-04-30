<script setup lang="tsx">
import { Badge, Tag } from 'ant-design-vue'
import dayjs from 'dayjs'
import { CloudUploadOutlined, SafetyCertificateOutlined } from '@ant-design/icons-vue'
import { input } from '@/components/StdDesign/StdDataEntry'
import type { customRender } from '@/components/StdDesign/StdDataDisplay/StdTableTransformer'
import { datetime } from '@/components/StdDesign/StdDataDisplay/StdTableTransformer'
import cert from '@/api/cert'
import type { Column, JSXElements } from '@/components/StdDesign/types'
import type { Cert } from '@/api/cert'
import { AutoCertState } from '@/constants'
import StdTable from '@/components/StdDesign/StdDataDisplay/StdTable.vue'
import WildcardCertificate from '@/views/certificate/WildcardCertificate.vue'

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
    const template: JSXElements = []
    const { text } = args
    const managed = $gettext('Managed Certificate')
    const general = $gettext('General Certificate')
    if (text === true || text > 0) {
      template.push(<Tag bordered={false} color="processing">
        { managed }
      </Tag>)
    }

    else {
      template.push(<Tag bordered={false} color="purple">{
      general }
      </Tag>)
    }

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
  hiddenInTable: true,
}, {
  title: () => $gettext('SSL Certificate Key Path'),
  dataIndex: 'ssl_certificate_key_path',
  edit: {
    type: input,
    show: notShowInAutoCert,
  },
  hiddenInTable: true,
}, {
  title: () => $gettext('Status'),
  dataIndex: 'certificate_info',
  customRender: (args: customRender) => {
    const template: JSXElements = []

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

const refWildcard = ref()
const refTable = ref()
</script>

<template>
  <ACard :title="$gettext('Certificates')">
    <template #extra>
      <AButton
        type="link"
        @click="$router.push('/certificates/import')"
      >
        <CloudUploadOutlined />
        {{ $gettext('Import') }}
      </AButton>

      <AButton
        type="link"
        @click="() => refWildcard.open()"
      >
        <SafetyCertificateOutlined />
        {{ $gettext('Issue wildcard certificate') }}
      </AButton>
    </template>
    <StdTable
      ref="refTable"
      :api="cert"
      :columns="columns"
      @click-edit="id => $router.push(`/certificates/${id}`)"
    />
    <WildcardCertificate
      ref="refWildcard"
      @issued="() => refTable.get_list()"
    />
  </ACard>
</template>

<style lang="less" scoped>

</style>
