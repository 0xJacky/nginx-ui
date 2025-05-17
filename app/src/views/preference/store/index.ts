import type { Settings } from '@/api/settings'
import { message } from 'ant-design-vue'
import { defineStore } from 'pinia'
import settings from '@/api/settings'
import { use2FAModal } from '@/components/TwoFA'
import { useSettingsStore } from '@/pinia'

const useSystemSettingsStore = defineStore('systemSettings', () => {
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
      container_name: '',
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
  const errors = ref<Record<string, Record<string, string>>>({})

  function getSettings() {
    settings.get().then(r => {
      data.value = r
    })
  }

  async function save() {
    if (!data.value)
      return

    // fix type
    data.value.cert.http_challenge_port = data.value.cert.http_challenge_port.toString()

    const otpModal = use2FAModal()

    otpModal.open().then(() => {
      settings.save(data.value!).then(r => {
        const settingsStore = useSettingsStore()
        const { server_name } = storeToRefs(settingsStore)
        if (!settingsStore.is_remote)
          server_name.value = r?.server?.name ?? ''
        data.value = r
        message.success($gettext('Save successfully'))
        errors.value = {}
      })
    })
  }

  return { data, errors, getSettings, save }
})

export default useSystemSettingsStore
