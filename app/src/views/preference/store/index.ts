import type { Settings } from '@/api/settings'
import { toRaw } from 'vue'
import settings from '@/api/settings'
import { TwoFACancelledError, use2FAModal } from '@/components/TwoFA'
import { useSettingsStore } from '@/pinia'

const saveTimeoutMs = 12000

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
      renewal_interval: 7,
      recursive_nameservers: [],
      http_challenge_port: '9180',
      discovery_patterns: [],
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
    nginx_log: {
      indexing_enabled: false,
      index_path: '',
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
  const saving = ref(false)
  let saveRequestTimer: ReturnType<typeof setTimeout> | undefined
  let saveController: AbortController | undefined
  let saveAttempt = 0

  function clearSaveRequestState() {
    if (saveRequestTimer)
      clearTimeout(saveRequestTimer)
    saveRequestTimer = undefined
    saveController = undefined
  }

  function buildSavePayload() {
    const payload = JSON.parse(JSON.stringify(toRaw(data.value))) as Settings
    payload.cert.http_challenge_port = payload.cert.http_challenge_port.toString()
    payload.cert.recursive_nameservers = (payload.cert.recursive_nameservers ?? [])
      .map(nameserver => nameserver.trim())
      .filter(Boolean)
    payload.cert.discovery_patterns = (payload.cert.discovery_patterns ?? [])
      .map(pattern => pattern.trim())
      .filter(Boolean)
    return payload
  }

  function getSettings() {
    settings.get().then(r => {
      r.cert.recursive_nameservers ||= []
      r.cert.discovery_patterns ||= []
      data.value = r
    })
  }

  async function save() {
    if (!data.value)
      return

    if (saveController)
      saveController.abort()
    clearSaveRequestState()

    const currentAttempt = ++saveAttempt

    let payload: Settings
    try {
      payload = buildSavePayload()
    }
    catch (error) {
      console.error(error)
      saving.value = false
      const errorMessage = $gettext('Failed to save configuration')
      message.error(errorMessage)
      return
    }

    const otpModal = use2FAModal()
    try {
      await otpModal.open()
    }
    catch (error) {
      if (currentAttempt !== saveAttempt)
        return

      if (error instanceof TwoFACancelledError)
        return

      console.error(error)
      const errorMessage = $gettext('Failed to save configuration')
      message.error(errorMessage)
      return
    }

    if (currentAttempt !== saveAttempt)
      return

    saving.value = true
    const controller = new AbortController()
    saveController = controller

    saveRequestTimer = setTimeout(() => {
      if (currentAttempt !== saveAttempt || !saving.value)
        return

      controller.abort()
      saving.value = false
      const errorMessage = $gettext('Failed to save configuration')
      message.error(errorMessage)
      clearSaveRequestState()
    }, saveTimeoutMs)

    setTimeout(() => {
      settings.save(payload, { signal: controller.signal }).then(r => {
        if (currentAttempt !== saveAttempt)
          return

        clearSaveRequestState()
        const settingsStore = useSettingsStore()
        const { server_name } = storeToRefs(settingsStore)
        if (!settingsStore.is_remote)
          server_name.value = r?.server?.name ?? ''
        r.cert.recursive_nameservers ||= []
        r.cert.discovery_patterns ||= []
        data.value = r
        saving.value = false
        message.success($gettext('Save successfully'))
        errors.value = {}
      }).catch(error => {
        if (currentAttempt !== saveAttempt || controller.signal.aborted)
          return

        clearSaveRequestState()
        console.error(error)
        saving.value = false
        const errorMessage = $gettext('Failed to save configuration')
        message.error(errorMessage)
      }).finally(() => {
        if (currentAttempt !== saveAttempt)
          return

        clearSaveRequestState()
      })
    }, 0)
  }

  return { data, errors, saving, getSettings, save }
})

export default useSystemSettingsStore
