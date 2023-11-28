<script setup lang="tsx">
import { useGettext } from 'vue3-gettext'
import { Badge } from 'ant-design-vue'
import { h } from 'vue'
import { input } from '@/components/StdDesign/StdDataEntry'
import type { customRender } from '@/components/StdDesign/StdDataDisplay/StdTableTransformer'
import { datetime } from '@/components/StdDesign/StdDataDisplay/StdTableTransformer'
import cert from '@/api/cert'
import StdCurd from '@/components/StdDesign/StdDataDisplay/StdCurd.vue'
import CodeEditor from '@/components/CodeEditor/CodeEditor.vue'
import CertInfo from '@/views/domain/cert/CertInfo.vue'

const { $gettext, interpolate } = useGettext()

const columns = [{
  title: () => $gettext('Name'),
  dataIndex: 'name',
  sorter: true,
  pithy: true,
  customRender: (args: customRender) => {
    const { text, record } = args
    if (!text)
      return h('div', record.domain)

    return h('div', text)
  },
  edit: {
    type: input,
  },
  search: true,
}, {
  title: () => $gettext('Config Name'),
  dataIndex: 'filename',
  sorter: true,
  pithy: true,
}, {
  title: () => $gettext('Auto Cert'),
  dataIndex: 'auto_cert',
  customRender: (args: customRender) => {
    const template = []
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
  sorter: true,
  pithy: true,
}, {
  title: () => $gettext('SSL Certificate Path'),
  dataIndex: 'ssl_certificate_path',
  edit: {
    type: input,
  },
  display: false,
}, {
  title: () => $gettext('SSL Certificate Key Path'),
  dataIndex: 'ssl_certificate_key_path',
  edit: {
    type: input,
  },
  display: false,
}, {
  title: () => $gettext('Updated at'),
  dataIndex: 'updated_at',
  customRender: datetime,
  sorter: true,
  pithy: true,
}, {
  title: () => $gettext('Action'),
  dataIndex: 'action',
}]
</script>

<template>
  <StdCurd
    :title="$gettext('Certification')"
    :api="cert"
    :columns="columns"
    row-key="name"
  >
    <template #beforeEdit="{ data }">
      <template v-if="data.auto_cert === 1">
        <div style="margin-bottom: 15px">
          <AAlert
            :message="$gettext('Auto cert is enabled, please do not modify this certification.')"
            type="info"
            show-icon
          />
        </div>
        <div
          v-if="!data.filename"
          style="margin-bottom: 15px"
        >
          <AAlert
            :message="$gettext('This auto-cert item is invalid, please remove it.')"
            type="error"
            show-icon
          />
        </div>
        <div
          v-else-if="!data.domains"
          style="margin-bottom: 15px"
        >
          <AAlert
            :message="interpolate($gettext('Domains list is empty, try to reopen auto-cert for %{config}'), { config: data.filename })"
            type="error"
            show-icon
          />
        </div>
        <div
          v-if="data.log"
          style="margin-bottom: 15px"
        >
          <AForm layout="vertical">
            <AFormItem :label="$gettext('Auto-Cert Log')">
              <p>{{ data.log }}</p>
            </AFormItem>
          </AForm>
        </div>
      </template>
      <AForm
        v-if="data.certificate_info"
        layout="vertical"
      >
        <AFormItem :label="$gettext('Certificate Status')">
          <CertInfo :cert="data.certificate_info" />
        </AFormItem>
      </AForm>
    </template>
    <template #edit="{ data }">
      <AForm layout="vertical">
        <AFormItem :label="$gettext('SSL Certification Content')">
          <CodeEditor
            v-model:content="data.ssl_certification"
            default-height="200px"
          />
        </AFormItem>
        <AFormItem :label="$gettext('SSL Certification Key Content')">
          <CodeEditor
            v-model:content="data.ssl_certification_key"
            default-height="200px"
          />
        </AFormItem>
      </AForm>
    </template>
  </StdCurd>
</template>

<style lang="less" scoped>

</style>
