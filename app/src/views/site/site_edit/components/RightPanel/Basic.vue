<script setup lang="ts">
import type { SiteStatus } from '@/api/site'
import { InfoCircleOutlined } from '@ant-design/icons-vue'
import { StdSelector } from '@uozi-admin/curd'
import envGroup from '@/api/env_group'
import NodeSelector from '@/components/NodeSelector'
import { formatDateTime } from '@/lib/helper'
import { useSettingsStore } from '@/pinia'
import envGroupColumns from '@/views/environments/group/columns'
import SiteStatusSelect from '@/views/site/components/SiteStatusSelect.vue'
import ConfigName from '@/views/site/site_edit/components/ConfigName/ConfigName.vue'
import { useSiteEditorStore } from '../SiteEditor/store'

const settings = useSettingsStore()

const editorStore = useSiteEditorStore()
const { name, data } = storeToRefs(editorStore)

function handleStatusChanged(event: { status: SiteStatus }) {
  data.value.status = event.status
}
</script>

<template>
  <div class="px-6">
    <div class="mb-6">
      <AForm layout="vertical">
        <AFormItem :label="$gettext('Status')">
          <SiteStatusSelect
            v-model="data.status"
            :site-name="name"
            @status-changed="handleStatusChanged"
          />
        </AFormItem>
        <AFormItem :label="$gettext('Name')">
          <ConfigName v-if="name" :name />
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
      </AForm>
    </div>

    <div v-if="!settings.is_remote">
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

<style scoped lang="less">
:deep(.ant-collapse-ghost > .ant-collapse-item > .ant-collapse-content > .ant-collapse-content-box) {
  padding: 0;
}

:deep(.ant-collapse > .ant-collapse-item > .ant-collapse-header) {
  padding: 0 0 10px 0;
}
</style>
