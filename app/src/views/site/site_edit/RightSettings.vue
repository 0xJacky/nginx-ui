<script setup lang="ts">
import type { ChatComplicationMessage } from '@/api/openai'
import type { Site, SiteStatus } from '@/api/site'
import type { Ref } from 'vue'
import envGroup from '@/api/env_group'
import ChatGPT from '@/components/ChatGPT/ChatGPT.vue'
import NodeSelector from '@/components/NodeSelector/NodeSelector.vue'
import StdSelector from '@/components/StdDesign/StdDataEntry/components/StdSelector.vue'
import { formatDateTime } from '@/lib/helper'
import { useSettingsStore } from '@/pinia'
import envGroupColumns from '@/views/environments/group/columns'
import SiteStatusSegmented from '@/views/site/components/SiteStatusSegmented.vue'
import ConfigName from '@/views/site/site_edit/components/ConfigName.vue'
import { InfoCircleOutlined } from '@ant-design/icons-vue'

const settings = useSettingsStore()

const configText = inject('configText') as Ref<string>
const name = inject('name') as ComputedRef<string>
const filepath = inject('filepath') as Ref<string>
const historyChatgptRecord = inject('history_chatgpt_record') as Ref<ChatComplicationMessage[]>
const data = inject('data') as Ref<Site>

const activeKey = ref(['1', '2', '3'])

function handleStatusChanged(event: { status: SiteStatus }) {
  data.value.status = event.status
}
</script>

<template>
  <ACard
    class="right-settings"
    :bordered="false"
  >
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
          <AFormItem :label="$gettext('Status')">
            <SiteStatusSegmented
              v-model="data.status"
              :site-name="name"
              @status-changed="handleStatusChanged"
            />
          </AFormItem>
          <AFormItem :label="$gettext('Name')">
            <ConfigName v-if="name" :name />
          </AFormItem>
          <AFormItem :label="$gettext('Node Group')">
            <StdSelector
              v-model:selected-key="data.env_group_id"
              :api="envGroup"
              :columns="envGroupColumns"
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
