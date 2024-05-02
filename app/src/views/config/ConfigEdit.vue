<script setup lang="ts">
import { useRoute } from 'vue-router'
import { message } from 'ant-design-vue'
import type { Ref } from 'vue'
import { formatDateTime } from '@/lib/helper'
import FooterToolBar from '@/components/FooterToolbar/FooterToolBar.vue'
import config from '@/api/config'
import CodeEditor from '@/components/CodeEditor/CodeEditor.vue'
import ngx from '@/api/ngx'
import InspectConfig from '@/views/config/InspectConfig.vue'
import ChatGPT from '@/components/ChatGPT/ChatGPT.vue'
import type { ChatComplicationMessage } from '@/api/openai'

const route = useRoute()

const inspect_config = ref()

const name = computed(() => {
  const n = route.params.name
  if (typeof n === 'string')
    return n

  return n?.join('/')
})

const configText = ref('')
const history_chatgpt_record = ref([]) as Ref<ChatComplicationMessage[]>
const filepath = ref('')
const active_key = ref(['1', '2'])
const modified_at = ref('')

function init() {
  if (name.value) {
    config.get(name.value).then(r => {
      configText.value = r.content
      history_chatgpt_record.value = r.chatgpt_messages
      filepath.value = r.filepath
      modified_at.value = r.modified_at
    }).catch(r => {
      message.error(r.message ?? $gettext('Server error'))
    })
  }
  else {
    configText.value = ''
    history_chatgpt_record.value = []
    filepath.value = ''
  }
}

init()

function save() {
  config.save(name.value, { content: configText.value }).then(r => {
    configText.value = r.content
    message.success($gettext('Saved successfully'))
  }).catch(r => {
    message.error($gettext('Save error %{msg}', { msg: r.message ?? '' }))
  }).finally(() => {
    inspect_config.value.test()
  })
}

function format_code() {
  ngx.format_code(configText.value).then(r => {
    configText.value = r.content
    message.success($gettext('Format successfully'))
  }).catch(r => {
    message.error($gettext('Format error %{msg}', { msg: r.message ?? '' }))
  })
}

</script>

<template>
  <ARow :gutter="16">
    <ACol
      :xs="24"
      :sm="24"
      :md="18"
    >
      <ACard :title="$gettext('Edit Configuration')">
        <InspectConfig ref="inspect_config" />
        <CodeEditor v-model:content="configText" />
        <FooterToolBar>
          <ASpace>
            <AButton @click="$router.go(-1)">
              {{ $gettext('Back') }}
            </AButton>
            <AButton @click="format_code">
              {{ $gettext('Format Code') }}
            </AButton>
            <AButton
              type="primary"
              @click="save"
            >
              {{ $gettext('Save') }}
            </AButton>
          </ASpace>
        </FooterToolBar>
      </ACard>
    </ACol>

    <ACol
      :xs="24"
      :sm="24"
      :md="6"
    >
      <ACard class="col-right">
        <ACollapse
          v-model:activeKey="active_key"
          ghost
        >
          <ACollapsePanel
            key="1"
            :header="$gettext('Basic')"
          >
            <AForm layout="vertical">
              <AFormItem :label="$gettext('Path')">
                {{ filepath }}
              </AFormItem>
              <AFormItem :label="$gettext('Updated at')">
                {{ formatDateTime(modified_at) }}
              </AFormItem>
            </AForm>
          </ACollapsePanel>
          <ACollapsePanel
            key="2"
            header="ChatGPT"
          >
            <ChatGPT
              v-model:history-messages="history_chatgpt_record"
              :content="configText"
              :path="filepath"
            />
          </ACollapsePanel>
        </ACollapse>
      </ACard>
    </ACol>
  </ARow>
</template>

<style lang="less" scoped>
.col-right {
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
