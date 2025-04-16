<script setup lang="ts">
import type { Settings } from '@/api/settings'
import settings from '@/api/settings'
import FooterToolBar from '@/components/FooterToolbar/FooterToolBar.vue'
import use2FAModal from '@/components/TwoFA/use2FAModal'
import { useSettingsStore } from '@/pinia'
import AppSettings from '@/views/preference/AppSettings.vue'
import AuthSettings from '@/views/preference/AuthSettings.vue'
import CertSettings from '@/views/preference/CertSettings.vue'
import ExternalNotify from '@/views/preference/ExternalNotify.vue'
import HTTPSettings from '@/views/preference/HTTPSettings.vue'
import LogrotateSettings from '@/views/preference/LogrotateSettings.vue'
import NginxSettings from '@/views/preference/NginxSettings.vue'
import NodeSettings from '@/views/preference/NodeSettings.vue'
import OpenAISettings from '@/views/preference/OpenAISettings.vue'
import ServerSettings from '@/views/preference/ServerSettings.vue'
import TerminalSettings from '@/views/preference/TerminalSettings.vue'
import { message } from 'ant-design-vue'
import { storeToRefs } from 'pinia'

const data = ref<Settings>({
  app: {
    page_size: 10,
    jwt_secret: '',
  },
  server: {
    host: '0.0.0.0',
    port: 9000,
    run_mode: 'debug',
    enable_https: false,
    ssl_cert: '',
    ssl_key: '',
  },
  database: {
    name: '',
  },
  auth: {
    ip_white_list: [],
    ban_threshold_minutes: 10,
    max_attempts: 10,
  },
  casdoor: {
    endpoint: '',
    client_id: '',
    client_secret: '',
    certificate_path: '',
    organization: '',
    application: '',
    redirect_uri: '',
  },
  cert: {
    email: '',
    ca_dir: '',
    renewal_interval: 7,
    recursive_nameservers: [],
    http_challenge_port: '9180',
  },
  http: {
    github_proxy: '',
    insecure_skip_verify: false,
  },
  logrotate: {
    enabled: false,
    cmd: '',
    interval: 1440,
  },
  nginx: {
    access_log_path: '',
    error_log_path: '',
    config_dir: '',
    config_path: '',
    log_dir_white_list: [],
    pid_path: '',
    test_config_cmd: '',
    reload_cmd: '',
    restart_cmd: '',
    stub_status_port: 51820,
  },
  node: {
    name: '',
    secret: '',
    skip_installation: false,
    demo: false,
    icp_number: '',
    public_security_number: '',
  },
  openai: {
    model: '',
    base_url: '',
    proxy: '',
    token: '',
    api_type: 'OPEN_AI',
    enable_code_completion: false,
    code_completion_model: '',
  },
  terminal: {
    start_cmd: '',
  },
  webauthn: {
    rp_display_name: '',
    rpid: '',
    rp_origins: [],
  },
})

settings.get().then(r => {
  data.value = r
})

const settingsStore = useSettingsStore()
const { server_name } = storeToRefs(settingsStore)
const errors = ref<Record<string, Record<string, string>>>({})
const refAuthSettings = useTemplateRef('refAuthSettings')

async function save() {
  // fix type
  data.value.cert.http_challenge_port = data.value.cert.http_challenge_port.toString()

  const otpModal = use2FAModal()

  otpModal.open().then(() => {
    settings.save(data.value).then(r => {
      if (!settingsStore.is_remote)
        server_name.value = r?.server?.name ?? ''
      data.value = r
      refAuthSettings.value?.getBannedIPs?.()
      message.success($gettext('Save successfully'))
      errors.value = {}
    })
  })
}

provide('data', data)
provide('errors', errors)

const router = useRouter()
const route = useRoute()
const activeKey = ref('server')

watch(activeKey, () => {
  router.push({
    query: {
      tab: activeKey.value,
    },
  })
})

onMounted(() => {
  if (route.query?.tab)
    activeKey.value = route.query.tab.toString()
})
</script>

<template>
  <ACard :title="$gettext('Preference')">
    <div class="preference-container">
      <ATabs v-model:active-key="activeKey">
        <ATabPane
          key="server"
          :tab="$gettext('Server')"
        >
          <ServerSettings />
        </ATabPane>
        <ATabPane
          key="app"
          :tab="$gettext('App')"
        >
          <AppSettings />
        </ATabPane>
        <ATabPane
          key="external_notify"
          :tab="$gettext('External Notify')"
        >
          <ExternalNotify />
        </ATabPane>
        <ATabPane
          key="node"
          :tab="$gettext('Node')"
        >
          <NodeSettings />
        </ATabPane>
        <ATabPane
          key="http"
          :tab="$gettext('HTTP')"
        >
          <HTTPSettings />
        </ATabPane>
        <ATabPane
          key="terminal"
          :tab="$gettext('Terminal')"
        >
          <TerminalSettings />
        </ATabPane>
        <ATabPane
          key="auth"
          :tab="$gettext('Auth')"
        >
          <AuthSettings ref="refAuthSettings" />
        </ATabPane>
        <ATabPane
          key="cert"
          :tab="$gettext('Cert')"
        >
          <CertSettings />
        </ATabPane>
        <ATabPane
          key="nginx"
          :tab="$gettext('Nginx')"
        >
          <NginxSettings />
        </ATabPane>
        <ATabPane
          key="openai"
          :tab="$gettext('OpenAI')"
        >
          <OpenAISettings />
        </ATabPane>
        <ATabPane
          key="logrotate"
          :tab="$gettext('Logrotate')"
        >
          <LogrotateSettings />
        </ATabPane>
      </ATabs>
    </div>
    <FooterToolBar v-if="activeKey !== 'external_notify'">
      <AButton
        type="primary"
        @click="save"
      >
        {{ $gettext('Save') }}
      </AButton>
    </FooterToolBar>
  </ACard>
</template>

<style lang="less" scoped>
.preference-container {
  width: 100%;
  max-width: 850px;
  margin: 0 auto;
  padding: 0 10px;
}
</style>
