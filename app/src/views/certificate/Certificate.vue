<script setup lang="tsx">
import { useGettext } from 'vue3-gettext'
import { Badge } from 'ant-design-vue'
import { h, provide } from 'vue'
import { input } from '@/components/StdDesign/StdDataEntry'
import type { customRender } from '@/components/StdDesign/StdDataDisplay/StdTableTransformer'
import { datetime } from '@/components/StdDesign/StdDataDisplay/StdTableTransformer'
import cert from '@/api/cert'
import StdCurd from '@/components/StdDesign/StdDataDisplay/StdCurd.vue'
import CodeEditor from '@/components/CodeEditor/CodeEditor.vue'
import CertInfo from '@/views/domain/cert/CertInfo.vue'
import type { Column } from '@/components/StdDesign/types'
import type { Cert } from '@/api/cert'
import { AutoCertState } from '@/constants'
import AutoCertStepOne from '@/views/domain/cert/components/AutoCertStepOne.vue'

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
  title: () => $gettext('Config Name'),
  dataIndex: 'filename',
  sortable: true,
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
  title: () => $gettext('Updated at'),
  dataIndex: 'updated_at',
  customRender: datetime,
  sortable: true,
  pithy: true,
}, {
  title: () => $gettext('Action'),
  dataIndex: 'action',
}]

const no_server_name = computed(() => {
  return false
})

provide('no_server_name', no_server_name)
</script>

<template>
  <StdCurd
    :title="$gettext('Certificates')"
    :api="cert"
    :columns="columns"
    :modal-max-width="600"
  >
    <template #beforeEdit="{ data }: {data: Cert}">
      <template v-if="data.auto_cert === AutoCertState.Enable">
        <div class="mt-4 mb-4">
          <AAlert
            :message="$gettext('Auto Cert is enabled')"
            type="success"
            show-icon
          />
        </div>
        <div
          v-if="!data.filename"
          class="mt-4 mb-4"
        >
          <AAlert
            :message="$gettext('This Auto Cert item is invalid, please remove it.')"
            type="error"
            show-icon
          />
        </div>
        <div
          v-else-if="!data.domains"
          class="mt-4 mb-4"
        >
          <AAlert
            :message="$gettext('Domains list is empty, try to reopen Auto Cert for %{config}', { config: data.filename })"
            type="error"
            show-icon
          />
        </div>
        <div
          v-if="data.log"
          class="mt-4 mb-4"
        >
          <AForm layout="vertical">
            <AFormItem :label="$gettext('Auto Cert Log')">
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

      <AutoCertStepOne hide-note />
    </template>
    <template #edit="{ data }: {data: Cert}">
      <AForm layout="vertical">
        <AFormItem :label="$gettext('SSL Certificate Content')">
          <CodeEditor
            v-model:content="data.ssl_certificate"
            default-height="200px"
            :readonly="!notShowInAutoCert(data)"
          />
        </AFormItem>
        <AFormItem :label="$gettext('SSL Certificate Key Content')">
          <CodeEditor
            v-model:content="data.ssl_certificate_key"
            default-height="200px"
            :readonly="!notShowInAutoCert(data)"
          />
        </AFormItem>
      </AForm>
    </template>
  </StdCurd>
</template>

<style lang="less" scoped>

</style>
