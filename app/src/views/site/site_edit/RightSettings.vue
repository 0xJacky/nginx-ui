<script setup lang="ts">
import type { ChatComplicationMessage } from '@/api/openai'
import type { Site } from '@/api/site'
import type { CheckedType } from '@/types'
import type { Ref } from 'vue'
import site from '@/api/site'
import site_category from '@/api/site_category'
import ChatGPT from '@/components/ChatGPT/ChatGPT.vue'
import NodeSelector from '@/components/NodeSelector/NodeSelector.vue'
import StdSelector from '@/components/StdDesign/StdDataEntry/components/StdSelector.vue'
import { formatDateTime } from '@/lib/helper'
import { useSettingsStore } from '@/pinia'
import siteCategoryColumns from '@/views/site/site_category/columns'
import ConfigName from '@/views/site/site_edit/components/ConfigName.vue'
import { InfoCircleOutlined } from '@ant-design/icons-vue'
import { message, Modal } from 'ant-design-vue'

const settings = useSettingsStore()

const configText = inject('configText') as Ref<string>
const enabled = inject('enabled') as Ref<boolean>
const name = inject('name') as ComputedRef<string>
const filepath = inject('filepath') as Ref<string>
const historyChatgptRecord = inject('history_chatgpt_record') as Ref<ChatComplicationMessage[]>
const data = inject('data') as Ref<Site>

const [modal, ContextHolder] = Modal.useModal()

const activeKey = ref(['1', '2', '3'])

function enable() {
  site.enable(name.value).then(() => {
    message.success($gettext('Enabled successfully'))
    enabled.value = true
  }).catch(r => {
    message.error($gettext('Failed to enable %{msg}', { msg: r.message ?? '' }), 10)
  })
}

function disable() {
  site.disable(name.value).then(() => {
    message.success($gettext('Disabled successfully'))
    enabled.value = false
  }).catch(r => {
    message.error($gettext('Failed to disable %{msg}', { msg: r.message ?? '' }))
  })
}

function onChangeEnabled(checked: CheckedType) {
  modal.confirm({
    title: checked ? $gettext('Do you want to enable this site?') : $gettext('Do you want to disable this site?'),
    mask: false,
    centered: true,
    okText: $gettext('OK'),
    cancelText: $gettext('Cancel'),
    async onOk() {
      if (checked)
        enable()
      else
        disable()
    },
  })
}
</script>

<template>
  <ACard
    class="right-settings"
    :bordered="false"
  >
    <ContextHolder />
    <ACollapse
      v-model:active-key="activeKey"
      ghost
      collapsible="header"
    >
      <ACollapsePanel
        key="1"
        :header="$gettext('Basic')"
      >
        <AForm layout="vertical">
          <AFormItem :label="$gettext('Enabled')">
            <ASwitch
              :checked="enabled"
              @change="onChangeEnabled"
            />
          </AFormItem>
          <AFormItem :label="$gettext('Name')">
            <ConfigName v-if="name" :name />
          </AFormItem>
          <AFormItem :label="$gettext('Category')">
            <StdSelector
              v-model:selected-key="data.site_category_id"
              :api="site_category"
              :columns="siteCategoryColumns"
              record-value-index="name"
              selection-type="radio"
            />
          </AFormItem>
          <AFormItem :label="$gettext('Updated at')">
            {{ formatDateTime(data.modified_at) }}
          </AFormItem>
        </AForm>
      </ACollapsePanel>
      <ACollapsePanel
        v-if="!settings.is_remote"
        key="2"
      >
        <template #header>
          {{ $gettext('Synchronization') }}
        </template>
        <template #extra>
          <APopover placement="bottomRight" :title="$gettext('Sync strategy')">
            <template #content>
              <div class="max-w-200px mb-2">
                {{ $gettext('When you enable/disable, delete, or save this site, '
                  + 'the nodes set in the site category and the nodes selected below will be synchronized.') }}
              </div>
              <div class="max-w-200px">
                {{ $gettext('Note, if the configuration file include other configurations or certificates, '
                  + 'please synchronize them to the remote nodes in advance.') }}
              </div>
            </template>
            <div class="text-trueGray-600">
              <InfoCircleOutlined class="mr-1" />
              {{ $gettext('Sync strategy') }}
            </div>
          </APopover>
        </template>
        <NodeSelector
          v-model:target="data.sync_node_ids"
          class="mb-4"
          hidden-local
        />
      </ACollapsePanel>
      <ACollapsePanel
        key="3"
        header="ChatGPT"
      >
        <ChatGPT
          v-model:history-messages="historyChatgptRecord"
          :content="configText"
          :path="filepath"
        />
      </ACollapsePanel>
    </ACollapse>
  </ACard>
</template>

<style scoped lang="less">
.right-settings {
  position: sticky;
  top: 78px;

  :deep(.ant-card-body) {
    max-height: 100vh;
    overflow-y: scroll;
  }
}

:deep(.ant-collapse-ghost > .ant-collapse-item > .ant-collapse-content > .ant-collapse-content-box) {
  padding: 0;
}

:deep(.ant-collapse > .ant-collapse-item > .ant-collapse-header) {
  padding: 0 0 10px 0;
}
</style>
