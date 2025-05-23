<script setup lang="ts">
import type { CheckedType } from '@/types'
import { InfoCircleOutlined } from '@ant-design/icons-vue'
import { StdSelector } from '@uozi-admin/curd'
import { message, Modal } from 'ant-design-vue'
import { storeToRefs } from 'pinia'
import envGroup from '@/api/env_group'
import stream from '@/api/stream'
import NodeSelector from '@/components/NodeSelector'
import { formatDateTime } from '@/lib/helper'
import { useSettingsStore } from '@/pinia'
import envGroupColumns from '@/views/environments/group/columns'
import { useStreamEditorStore } from '../../store'
import ConfigName from '../ConfigName.vue'

const settings = useSettingsStore()
const store = useStreamEditorStore()
const { name, enabled, data } = storeToRefs(store)

const [modal, ContextHolder] = Modal.useModal()
const showSync = computed(() => !settings.is_remote)

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
  <div>
    <ContextHolder />

    <AFormItem :label="$gettext('Enabled')">
      <ASwitch
        :checked="enabled"
        @change="onChangeEnabled"
      />
    </AFormItem>

    <AFormItem :label="$gettext('Name')">
      <ConfigName :name />
    </AFormItem>

    <AFormItem :label="$gettext('Updated at')">
      {{ formatDateTime(data.modified_at) }}
    </AFormItem>

    <AFormItem :label="$gettext('Node Group')">
      <StdSelector
        v-model:value="data.env_group_id"
        :get-list-api="envGroup.getList"
        :columns="envGroupColumns"
        display-key="name"
        selection-type="radio"
      />
    </AFormItem>
    <!-- Synchronization Section -->
    <div v-if="showSync" class="mt-4">
      <div class="flex items-center justify-between mb-4">
        <div>
          {{ $gettext('Synchronization') }}
        </div>
        <APopover placement="bottomRight" :title="$gettext('Sync strategy')">
          <template #content>
            <div class="max-w-200px mb-2">
              {{ $gettext('When you enable/disable, delete, or save this site, '
                + 'the nodes set in the Node Group and the nodes selected below will be synchronized.') }}
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
      </div>
      <NodeSelector
        v-model:target="data.sync_node_ids"
        class="mb-4"
        hidden-local
      />
    </div>
  </div>
</template>
