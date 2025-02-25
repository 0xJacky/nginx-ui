<script setup lang="ts">
import type { NgxConfig } from '@/api/ngx'
import type { ChatComplicationMessage } from '@/api/openai'
import type { Stream } from '@/api/stream'
import type { CheckedType } from '@/types'

import type { Ref } from 'vue'
import config from '@/api/config'
import ngx from '@/api/ngx'
import stream from '@/api/stream'
import CodeEditor from '@/components/CodeEditor/CodeEditor.vue'
import FooterToolBar from '@/components/FooterToolbar/FooterToolBar.vue'
import NgxConfigEditor from '@/views/site/ngx_conf/NgxConfigEditor.vue'
import RightSettings from '@/views/stream/components/RightSettings.vue'
import { message } from 'ant-design-vue'

const route = useRoute()
const router = useRouter()

const name = ref(route.params.name.toString())

watch(route, () => {
  name.value = route.params?.name?.toString() ?? ''
})

const ngxConfig: NgxConfig = reactive({
  name: '',
  upstreams: [],
  servers: [],
})

const enabled = ref(false)
const configText = ref('')
const advanceModeRef = ref(false)
const saving = ref(false)
const filename = ref('')
const filepath = ref('')
const parseErrorStatus = ref(false)
const parseErrorMessage = ref('')
const data = ref<Stream>({} as Stream)

init()

const advanceMode = computed({
  get() {
    return advanceModeRef.value || parseErrorStatus.value
  },
  set(v: boolean) {
    advanceModeRef.value = v
  },
})

const historyChatgptRecord = ref([]) as Ref<ChatComplicationMessage[]>

function handleResponse(r: Stream) {
  if (r.advanced)
    advanceMode.value = true

  if (r.advanced)
    advanceMode.value = true

  parseErrorStatus.value = false
  parseErrorMessage.value = ''
  filename.value = r.name
  filepath.value = r.filepath
  configText.value = r.config
  enabled.value = r.enabled
  historyChatgptRecord.value = r.chatgpt_messages
  data.value = r
  Object.assign(ngxConfig, r.tokenized)
}

function init() {
  if (name.value) {
    stream.get(name.value).then(r => {
      handleResponse(r)
    }).catch(handleParseError)
  }
  else {
    historyChatgptRecord.value = []
  }
}

function handleParseError(e: { error?: string, message: string }) {
  console.error(e)
  parseErrorStatus.value = true
  parseErrorMessage.value = e.message
  config.get(`streams-available/${name.value}`).then(r => {
    configText.value = r.content
  })
}

function onModeChange(advanced: CheckedType) {
  stream.advance_mode(name.value, { advanced: advanced as boolean }).then(() => {
    advanceMode.value = advanced as boolean
    if (advanced) {
      buildConfig()
    }
    else {
      return ngx.tokenize_config(configText.value).then(r => {
        Object.assign(ngxConfig, r)
      }).catch(handleParseError)
    }
  })
}

async function buildConfig() {
  return ngx.build_config(ngxConfig).then(r => {
    configText.value = r.content
  })
}

async function save() {
  saving.value = true

  if (!advanceMode.value) {
    try {
      await buildConfig()
    }
    catch {
      saving.value = false
      message.error($gettext('Failed to save, syntax error(s) was detected in the configuration.'))

      return
    }
  }

  return stream.save(name.value, {
    name: filename.value || name.value,
    content: configText.value,
    overwrite: true,
    sync_node_ids: data.value?.sync_node_ids,
  }).then(r => {
    handleResponse(r)
    router.push({
      path: `/streams/${filename.value}`,
      query: route.query,
    })
    message.success($gettext('Saved successfully'))
  }).catch(handleParseError).finally(() => {
    saving.value = false
  })
}

provide('save_config', save)
provide('configText', configText)
provide('ngx_config', ngxConfig)
provide('history_chatgpt_record', historyChatgptRecord)
provide('enabled', enabled)
provide('name', name)
provide('filename', filename)
provide('filepath', filepath)
provide('data', data)
</script>

<template>
  <ARow :gutter="16">
    <ACol
      :xs="24"
      :sm="24"
      :md="18"
    >
      <ACard :bordered="false">
        <template #title>
          <span style="margin-right: 10px">{{ $gettext('Edit %{n}', { n: name }) }}</span>
          <ATag
            v-if="enabled"
            color="blue"
          >
            {{ $gettext('Enabled') }}
          </ATag>
          <ATag
            v-else
            color="orange"
          >
            {{ $gettext('Disabled') }}
          </ATag>
        </template>
        <template #extra>
          <div class="mode-switch">
            <div class="switch">
              <ASwitch
                size="small"
                :disabled="parseErrorStatus"
                :checked="advanceMode"
                @change="onModeChange"
              />
            </div>
            <template v-if="advanceMode">
              <div>{{ $gettext('Advance Mode') }}</div>
            </template>
            <template v-else>
              <div>{{ $gettext('Basic Mode') }}</div>
            </template>
          </div>
        </template>

        <Transition name="slide-fade">
          <div
            v-if="advanceMode"
            key="advance"
          >
            <div
              v-if="parseErrorStatus"
              class="parse-error-alert-wrapper"
            >
              <AAlert
                :message="$gettext('Nginx Configuration Parse Error')"
                :description="parseErrorMessage"
                type="error"
                show-icon
              />
            </div>
            <div>
              <CodeEditor v-model:content="configText" />
            </div>
          </div>

          <div
            v-else
            key="basic"
            class="domain-edit-container"
          >
            <NgxConfigEditor
              :enabled="enabled"
              context="stream"
              @callback="save"
            />
          </div>
        </Transition>
      </ACard>
    </ACol>

    <ACol
      class="col-right"
      :xs="24"
      :sm="24"
      :md="6"
    >
      <RightSettings />
    </ACol>

    <FooterToolBar>
      <ASpace>
        <AButton @click="$router.push('/streams')">
          {{ $gettext('Back') }}
        </AButton>
        <AButton
          type="primary"
          :loading="saving"
          @click="save"
        >
          {{ $gettext('Save') }}
        </AButton>
      </ASpace>
    </FooterToolBar>
  </ARow>
</template>

<style lang="less">

</style>

<style lang="less" scoped>
.col-right {
  position: relative;
}

.ant-card {
  margin: 10px 0;
  box-shadow: unset;
}

.mode-switch {
  display: flex;

  .switch {
    display: flex;
    align-items: center;
    margin-right: 5px;
  }
}

.parse-error-alert-wrapper {
  margin-bottom: 20px;
}

.domain-edit-container {
  max-width: 800px;
  margin: 0 auto;
}

.slide-fade-enter-active {
  transition: all .3s ease-in-out;
}

.slide-fade-leave-active {
  transition: all .3s cubic-bezier(1.0, 0.5, 0.8, 1.0);
}

.slide-fade-enter-from, .slide-fade-enter-to, .slide-fade-leave-to
  /* .slide-fade-leave-active for below version 2.1.8 */ {
  transform: translateX(10px);
  opacity: 0;
}

.directive-params-wrapper {
  margin: 10px 0;
}

.tab-content {
  padding: 10px;
}
</style>
