<script setup lang="ts">
import type { ChatComplicationMessage } from '@/api/openai'
import type { Stream } from '@/api/stream'
import type { CheckedType } from '@/types'
import type { Ref } from 'vue'
import stream from '@/api/stream'
import ChatGPT from '@/components/ChatGPT/ChatGPT.vue'
import NodeSelector from '@/components/NodeSelector/NodeSelector.vue'
import { formatDateTime } from '@/lib/helper'
import { useSettingsStore } from '@/pinia'
import { message, Modal } from 'ant-design-vue'
import ConfigName from './ConfigName.vue'

const settings = useSettingsStore()

const configText = inject('configText') as Ref<string>
const enabled = inject('enabled') as Ref<boolean>
const name = inject('name') as Ref<string>
const historyChatgptRecord = inject('history_chatgpt_record') as Ref<ChatComplicationMessage[]>
const filepath = inject('filepath') as Ref<string>
const data = inject('data') as Ref<Stream>

const [modal, ContextHolder] = Modal.useModal()

const active_key = ref(['1', '2', '3'])

function enable() {
  stream.enable(name.value).then(() => {
    message.success($gettext('Enabled successfully'))
    enabled.value = true
  }).catch(r => {
    message.error($gettext('Failed to enable %{msg}', { msg: r.message ?? '' }), 10)
  })
}

function disable() {
  stream.disable(name.value).then(() => {
    message.success($gettext('Disabled successfully'))
    enabled.value = false
  }).catch(r => {
    message.error($gettext('Failed to disable %{msg}', { msg: r.message ?? '' }))
  })
}

function onChangeEnabled(checked: CheckedType) {
  modal.confirm({
    title: checked ? $gettext('Do you want to enable this stream?') : $gettext('Do you want to disable this stream?'),
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
      v-model:active-key="active_key"
      ghost
    >
      <ACollapsePanel
        key="1"
        :header="$gettext('Basic')"
      >
        <AFormItem :label="$gettext('Enabled')">
          <ASwitch
            :checked="enabled"
            @change="onChangeEnabled"
          />
        </AFormItem>
        <AFormItem :label="$gettext('Name')">
          <ConfigName :name="name" />
        </AFormItem>
        <AFormItem :label="$gettext('Updated at')">
          {{ formatDateTime(data.modified_at) }}
        </AFormItem>
      </ACollapsePanel>
      <ACollapsePanel
        v-if="!settings.is_remote"
        key="2"
        :header="$gettext('Sync')"
      >
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
