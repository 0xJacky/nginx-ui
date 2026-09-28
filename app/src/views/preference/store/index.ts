import type { SavableSettingsSection, Settings } from '@/api/settings'
import type { CosyError } from '@/lib/http/types'
import settings from '@/api/settings'
import { use2FAModal } from '@/components/TwoFA'
import { useGlobalApp } from '@/composables/useGlobalApp'
import { isTwoFactorCancelled, translateError } from '@/lib/http/error'
import { normalizeHttpError } from '@/lib/http/normalizeError'
import { useSettingsStore } from '@/pinia'
import { tabSettingsSections } from '../sections'

// Flattens a validation error map such as {ip_white_list: {0: "ip"}} into the
// paths of the invalid fields.
function invalidFieldPaths(errors: unknown, prefix = ''): string[] {
  if (!errors || typeof errors !== 'object')
    return prefix ? [prefix] : []

  return Object.entries(errors).flatMap(([key, value]) =>
    invalidFieldPaths(value, prefix ? `${prefix}.${key}` : key))
}

interface SectionSaveResult {
  section: SavableSettingsSection
  error?: string
}

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
    listener: {
      unix_socket: '',
      socket_mode: '',
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
      http_proxy: '',
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
      index_custom_mmdb: '',
      geo_map_path: '',
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
    site_check: {
      enabled: true,
      concurrency: 5,
      interval_seconds: 300,
    },
    upstream_check: {
      enabled: true,
      interval_seconds: 30,
    },
  })
  const errors = ref<Record<string, Record<string, string>>>({})
  const savedEnableHTTPS = ref(false)
  const isSaving = ref(false)
  const isLoaded = ref(false)

  async function getSettings(): Promise<boolean> {
    try {
      const r = await settings.get()
      r.cert.recursive_nameservers ||= []
      savedEnableHTTPS.value = r.server.enable_https
      data.value = r
      errors.value = {}
      isLoaded.value = true
      return true
    }
    catch (err) {
      console.error('Failed to load settings:', err)
      return false
    }
  }

  function normalizeBeforeSave() {
    data.value.cert.http_challenge_port = data.value.cert.http_challenge_port.toString()
    data.value.cert.recursive_nameservers = (data.value.cert.recursive_nameservers ?? [])
      .map(nameserver => nameserver.trim())
      .filter(Boolean)
  }

  async function saveSection(section: SavableSettingsSection): Promise<SectionSaveResult> {
    try {
      const r = await settings.saveSection(section, data.value[section], { skipErrHandling: true })
      if (section === 'cert')
        (r as Settings['cert']).recursive_nameservers ||= []
      data.value[section] = r as never
      delete errors.value[section]
      return { section }
    }
    catch (err) {
      if (isTwoFactorCancelled(err))
        return { section, error: '' }

      const cosyError = normalizeHttpError(err) as CosyError & { errors?: Record<string, string> }
      errors.value[section] = cosyError.errors ?? {}

      // Not every tab shows field errors inline, so name the fields
      const fields = invalidFieldPaths(cosyError.errors)
      if (fields.length)
        return { section, error: $gettext('Invalid settings: %{fields}', { fields: fields.join(', ') }) }

      return { section, error: await translateError(cosyError) }
    }
  }

  // Saves the settings edited on one preference tab. Each section on the tab
  // is its own request, so a failure is reported against the fields it
  // belongs to; changes left on other tabs are not touched.
  async function save(tab: string) {
    const sections = tabSettingsSections(tab)
    if (!data.value || isSaving.value || sections.length === 0)
      return

    normalizeBeforeSave()

    const otpModal = use2FAModal()

    try {
      await otpModal.open()
    }
    catch {
      // User cancelled 2FA or the preflight check failed — abort save silently
      return
    }

    const hasHTTPSChanged = sections.includes('server')
      && data.value.server.enable_https !== savedEnableHTTPS.value

    isSaving.value = true
    try {
      const results = await Promise.all(sections.map(saveSection))
      const failed = results.filter(result => result.error !== undefined)

      // A dismissed 2FA prompt is not an error worth reporting
      if (failed.some(result => result.error === ''))
        return

      if (failed.length) {
        failed.forEach(result => message.error(result.error!))
        return
      }

      const settingsStore = useSettingsStore()
      const { server_name } = storeToRefs(settingsStore)
      if (sections.includes('node') && !settingsStore.is_remote)
        server_name.value = data.value.node.name

      if (sections.includes('server')) {
        savedEnableHTTPS.value = data.value.server.enable_https

        const expectedProtocol = data.value.server.enable_https ? 'https:' : 'http:'
        if (hasHTTPSChanged && window.location.protocol !== expectedProtocol) {
          const redirectURL = new URL(window.location.href)
          redirectURL.protocol = expectedProtocol
          window.location.replace(redirectURL)
          return
        }
      }

      message.success($gettext('Save successfully'))
    }
    finally {
      isSaving.value = false
    }
  }

  return {
    data,
    errors,
    isSaving,
    isLoaded,
    getSettings,
    save,
  }
})

export default useSystemSettingsStore
