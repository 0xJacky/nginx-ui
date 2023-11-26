<script setup lang="ts">
import FooterToolBar from '@/components/FooterToolbar/FooterToolBar.vue'
import gettext from '@/gettext'
import {useRoute} from 'vue-router'
import {computed, ref} from 'vue'
import config from '@/api/config'
import {message} from 'ant-design-vue'
import CodeEditor from '@/components/CodeEditor/CodeEditor.vue'
import ngx from '@/api/ngx'
import InspectConfig from '@/views/config/InspectConfig.vue'
import ChatGPT from '@/components/ChatGPT/ChatGPT.vue'
import {formatDateTime} from '../../lib/helper'

const {$gettext, interpolate} = gettext
const route = useRoute()

const inspect_config = ref()

const name = computed(() => {
  const n = route.params.name
  if (typeof n === 'string') {
    return n
  }
  return n?.join('/')
})

const configText = ref('')
const history_chatgpt_record = ref([])
const file_path = ref('')
const active_key = ref(['1', '2'])
const modified_at = ref('')

function init() {
  if (name.value) {
    config.get(name.value).then(r => {
      configText.value = r.config
      history_chatgpt_record.value = r.chatgpt_messages
      file_path.value = r.file_path
      modified_at.value = r.modified_at
    }).catch(r => {
      message.error(r.message ?? $gettext('Server error'))
    })
  } else {
    configText.value = ''
    history_chatgpt_record.value = []
    file_path.value = ''
  }
}

init()

function save() {
  config.save(name.value, {content: configText.value}).then(r => {
    configText.value = r.config
    message.success($gettext('Saved successfully'))
  }).catch(r => {
    message.error(interpolate($gettext('Save error %{msg}'), {msg: r.message ?? ''}))
  }).finally(() => {
    inspect_config.value.test()
  })
}

function format_code() {
  ngx.format_code(configText.value).then(r => {
    configText.value = r.content
    message.success($gettext('Format successfully'))
  }).catch(r => {
    message.error(interpolate($gettext('Format error %{msg}'), {msg: r.message ?? ''}))
  })
}

</script>


<template>
  <a-row :gutter="16">
    <a-col :xs="24" :sm="24" :md="18">
      <a-card :title="$gettext('Edit Configuration')">
        <inspect-config ref="inspect_config"/>
        <code-editor v-model:content="configText"/>
        <footer-tool-bar>
          <a-space>
            <a-button @click="$router.go(-1)">
              <translate>Back</translate>
            </a-button>
            <a-button @click="format_code">
              <translate>Format Code</translate>
            </a-button>
            <a-button type="primary" @click="save">
              <translate>Save</translate>
            </a-button>
          </a-space>
        </footer-tool-bar>
      </a-card>
    </a-col>

    <a-col :xs="24" :sm="24" :md="6">
      <a-card class="col-right">
        <a-collapse v-model:activeKey="active_key" ghost>
          <a-collapse-panel key="1" :header="$gettext('Basic')">
            <a-form layout="vertical">
              <a-form-item :label="$gettext('Path')">
                {{ file_path }}
              </a-form-item>
              <a-form-item :label="$gettext('Updated at')">
                {{ formatDateTime(modified_at) }}
              </a-form-item>
            </a-form>
          </a-collapse-panel>
          <a-collapse-panel key="2" header="ChatGPT">
            <chat-g-p-t :content="configText" :path="file_path"
                        v-model:history_messages="history_chatgpt_record"/>
          </a-collapse-panel>
        </a-collapse>
      </a-card>
    </a-col>
  </a-row>
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
