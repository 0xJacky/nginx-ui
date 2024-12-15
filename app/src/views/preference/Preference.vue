<script setup lang="ts">
import type { Settings } from '@/api/settings'
import type { Ref } from 'vue'
import settings from '@/api/settings'
import FooterToolBar from '@/components/FooterToolbar/FooterToolBar.vue'
import use2FAModal from '@/components/TwoFA/use2FAModal'
import { useSettingsStore } from '@/pinia'
import AuthSettings from '@/views/preference/AuthSettings.vue'
import BasicSettings from '@/views/preference/BasicSettings.vue'
import CertSettings from '@/views/preference/CertSettings.vue'
import LogrotateSettings from '@/views/preference/LogrotateSettings.vue'
import NginxSettings from '@/views/preference/NginxSettings.vue'
import OpenAISettings from '@/views/preference/OpenAISettings.vue'
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
    log_dir_white_list: [],
    pid_path: '',
    reload_cmd: '',
    restart_cmd: '',
  },
  node: {
    name: '',
    secret: '',
    icp_number: '',
    public_security_number: '',
  },
  openai: {
    model: '',
    base_url: '',
    proxy: '',
    token: '',
    api_type: 'OPEN_AI',
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
const errors = ref({}) as Ref<Record<string, Record<string, string>>>
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
    }).catch(e => {
      errors.value = e.errors
      message.error(e?.message ?? $gettext('Server error'))
    })
  })
}

provide('data', data)
provide('errors', errors)

const router = useRouter()
const route = useRoute()
const activeKey = ref('basic')

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
          key="basic"
          :tab="$gettext('Basic')"
        >
          <BasicSettings />
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
    <FooterToolBar>
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
  max-width: 600px;
  margin: 0 auto;
  padding: 0 10px;
}
</style>
