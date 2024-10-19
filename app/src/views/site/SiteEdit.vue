<script setup lang="ts">
import { message } from 'ant-design-vue'
import FooterToolBar from '@/components/FooterToolbar/FooterToolBar.vue'
import CodeEditor from '@/components/CodeEditor/CodeEditor.vue'

import NgxConfigEditor from '@/views/site/ngx_conf/NgxConfigEditor.vue'
import type { Site } from '@/api/domain'
import domain from '@/api/domain'
import type { NgxConfig } from '@/api/ngx'
import ngx from '@/api/ngx'
import config from '@/api/config'
import RightSettings from '@/views/site/components/RightSettings.vue'
import type { CertificateInfo } from '@/api/cert'
import type { ChatComplicationMessage } from '@/api/openai'
import type { CheckedType } from '@/types'

const route = useRoute()
const router = useRouter()

const name = ref(route.params.name.toString())

watch(route, () => {
  name.value = route.params?.name?.toString() ?? ''
})

const ngx_config: NgxConfig = reactive({
  name: '',
  upstreams: [],
  servers: [],
})

const certInfoMap: Ref<Record<number, CertificateInfo[]>> = ref({})

const auto_cert = ref(false)
const enabled = ref(false)
const filepath = ref('')
const configText = ref('')
const advance_mode_ref = ref(false)
const saving = ref(false)
const filename = ref('')
const parse_error_status = ref(false)
const parse_error_message = ref('')
const data = ref({})

init()

const advance_mode = computed({
  get() {
    return advance_mode_ref.value || parse_error_status.value
  },
  set(v: boolean) {
    advance_mode_ref.value = v
  },
})

const history_chatgpt_record = ref([]) as Ref<ChatComplicationMessage[]>

function handle_response(r: Site) {
  if (r.advanced)
    advance_mode.value = true

  if (r.advanced)
    advance_mode.value = true

  parse_error_status.value = false
  parse_error_message.value = ''
  filename.value = r.name
  filepath.value = r.filepath
  configText.value = r.config
  enabled.value = r.enabled
  auto_cert.value = r.auto_cert
  history_chatgpt_record.value = r.chatgpt_messages
  data.value = r
  certInfoMap.value = r.cert_info || {}
  Object.assign(ngx_config, r.tokenized)
}

function init() {
  if (name.value) {
    domain.get(name.value).then(r => {
      handle_response(r)
    }).catch(handle_parse_error)
  }
  else {
    history_chatgpt_record.value = []
  }
}

function handle_parse_error(e: { error?: string; message: string }) {
  console.error(e)
  parse_error_status.value = true
  parse_error_message.value = e.message
  config.get(`sites-available/${name.value}`).then(r => {
    configText.value = r.content
  })
}

function on_mode_change(advanced: CheckedType) {
  domain.advance_mode(name.value, { advanced: advanced as boolean }).then(() => {
    advance_mode.value = advanced as boolean
    if (advanced) {
      build_config()
    }
    else {
      return ngx.tokenize_config(configText.value).then(r => {
        Object.assign(ngx_config, r)
      }).catch(handle_parse_error)
    }
  })
}

async function build_config() {
  return ngx.build_config(ngx_config).then(r => {
    configText.value = r.content
  })
}

const save = async () => {
  saving.value = true

  if (!advance_mode.value) {
    try {
      await build_config()
    }
    catch (e) {
      saving.value = false
      message.error($gettext('Failed to save, syntax error(s) was detected in the configuration.'))

      return
    }
  }

  return domain.save(name.value, {
    name: filename.value || name.value,
    content: configText.value,
    overwrite: true,
  }).then(r => {
    handle_response(r)
    router.push({
      path: `/sites/${filename.value}`,
      query: route.query,
    })
    message.success($gettext('Saved successfully'))
  }).catch(handle_parse_error).finally(() => {
    saving.value = false
  })
}

provide('save_config', save)
provide('configText', configText)
provide('ngx_config', ngx_config)
provide('history_chatgpt_record', history_chatgpt_record)
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
      :md="16"
      :lg="18"
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
                :disabled="parse_error_status"
                :checked="advance_mode"
                @change="on_mode_change"
              />
            </div>
            <template v-if="advance_mode">
              <div>{{ $gettext('Advance Mode') }}</div>
            </template>
            <template v-else>
              <div>{{ $gettext('Basic Mode') }}</div>
            </template>
          </div>
        </template>

        <Transition name="slide-fade">
          <div
            v-if="advance_mode"
            key="advance"
          >
            <div
              v-if="parse_error_status"
              class="parse-error-alert-wrapper"
            >
              <AAlert
                :message="$gettext('Nginx Configuration Parse Error')"
                :description="parse_error_message"
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
              v-model:auto-cert="auto_cert"
              :cert-info="certInfoMap"
              :enabled="enabled"
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
      :md="8"
      :lg="6"
    >
      <RightSettings />
    </ACol>

    <FooterToolBar>
      <ASpace>
        <AButton @click="$router.push('/sites/list')">
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
