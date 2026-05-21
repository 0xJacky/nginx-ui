import type { Settings } from '@/api/settings'
import settings from '@/api/settings'
import { use2FAModal } from '@/components/TwoFA'
import { useGlobalApp } from '@/composables/useGlobalApp'
import { useSettingsStore } from '@/pinia'

const useSystemSettingsStore = defineStore('systemSettings', () => {
  const { message } = useGlobalApp()

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
      enable_h2: false,
      enable_h3: false,
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
    oidc: {
      client_id: '',
      client_secret: '',
      endpoint: '',
      redirect_uri: '',
      scopes: '',
      identifier: '',
    },
    cert: {
      email: '',
      ca_dir: '',
      renewal_interval: 30,
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
      sbin_path: '',
      log_dir_white_list: [],
      pid_path: '',
      test_config_cmd: '',
      reload_cmd: '',
      restart_cmd: '',
      stub_status_port: 51820,
      container_name: '',
    },
    nginx_log: {
      indexing_enabled: false,
      index_path: '',
    },
    node: {
      name: '',
      secret: '',
      instance_id: '',
      skip_installation: false,
      demo: false,
      icp_number: '',
      public_security_number: '',
    },
    openai: {
      provider: 'openai',
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
  const savedEnableHTTPS = ref(false)

  function getSettings() {
    settings.get().then(r => {
      r.cert.recursive_nameservers ||= []
      savedEnableHTTPS.value = r.server.enable_https
      data.value = r
    }).catch(err => {
      console.error('Failed to load settings:', err)
    })
  }

  async function save() {
    if (!data.value)
      return

    // fix type
    data.value.cert.http_challenge_port = data.value.cert.http_challenge_port.toString()
    data.value.cert.recursive_nameservers = (data.value.cert.recursive_nameservers ?? [])
      .map(nameserver => nameserver.trim())
      .filter(Boolean)
    const hasHTTPSChanged = data.value.server.enable_https !== savedEnableHTTPS.value

    const otpModal = use2FAModal()

    try {
      await otpModal.open()
    }
    catch {
      // User cancelled 2FA or the preflight check failed — abort save silently
      return
    }

    try {
      const r = await settings.save(data.value!)
      const settingsStore = useSettingsStore()
      const { server_name } = storeToRefs(settingsStore)
      if (!settingsStore.is_remote)
        server_name.value = r?.server?.name ?? ''
      r.cert.recursive_nameservers ||= []
      savedEnableHTTPS.value = r.server.enable_https
      data.value = r
      errors.value = {}

      const expectedProtocol = r.server.enable_https ? 'https:' : 'http:'
      if (hasHTTPSChanged && window.location.protocol !== expectedProtocol) {
        const redirectURL = new URL(window.location.href)
        redirectURL.protocol = expectedProtocol
        window.location.replace(redirectURL)
        return
      }

      message.success($gettext('Save successfully'))
    }
    catch (err) {
      // The HTTP interceptor already surfaces the error via handleApiError,
      // so we only log here to avoid a duplicate toast.
      console.error('Failed to save settings:', err)
    }
  }

  return { data, errors, getSettings, save }
})

export default useSystemSettingsStore
