import type { DNSProviderForm } from '@/api/auto_cert'
import type { PluginTrust } from '@/api/plugin_marketplace'
import type { HttpConfig } from '@/lib/http/types'
import { http } from '@uozi-admin/request'

/**
 * Value the backend returns in place of a stored secret. Sending it back
 * unchanged keeps the value that is already stored.
 */
export const PLUGIN_SECRET_PLACEHOLDER = '__NGINX_UI_REDACTED__'

export type PluginStatus
  = | 'installed'
    | 'starting'
    | 'running'
    | 'stopped'
    | 'error'
    | 'missing'
    | 'incompatible'
    | 'needs_approval'

export type PluginLifecycle = 'resident' | 'on_demand'

/** How the host reaches a running plugin process for capability calls. */
export type PluginTransport = 'stdio' | 'grpc'

export type PluginSyncPolicy = 'manual' | 'auto'

/** `list` holds an array of single line strings. */
export type SettingsFieldType = 'text' | 'bool' | 'number' | 'select' | 'secret' | 'textarea' | 'list'

export interface SettingsOption {
  value: string
  label: string
}

export interface SettingsField {
  key: string
  type: SettingsFieldType
  display_name: string
  help_text?: string
  default?: unknown
  options?: SettingsOption[]
  required?: boolean
}

export interface SettingsSchema {
  header?: string
  footer?: string
  settings: SettingsField[]
}

export interface PluginRequirement {
  id: string
  version?: string
}

export interface PluginManifestServer {
  /** Maps "<goos>-<goarch>" to a path relative to the plugin directory. */
  executables?: Record<string, string>
  /** Fallback argv for interpreted plugins. */
  command?: string[]
  lifecycle?: PluginLifecycle
  idle_timeout_seconds?: number
  /** Resource hints, capped by the limits of the host. */
  resources?: PluginManifestResources
}

export interface PluginManifestResources {
  /** Memory in MiB, 0 means no hint. */
  memory_mb?: number
  /** CPU time in percent of one core, 0 means no hint. */
  cpu_percent?: number
}

export interface PluginManifestPage {
  path: string
  /** Locale code to page title. */
  title: Record<string, string>
  icon?: string
  file: string
}

export interface PluginManifestWebapp {
  bundle_path?: string
  style_path?: string
  /** Maps a shared runtime library to the semver range the bundle was built against. */
  shared?: Record<string, string>
  pages?: PluginManifestPage[]
}

export interface PluginManifestContent {
  templates?: string
  locales?: string
}

export interface PluginManifestCron {
  id: string
  schedule: string
  method: string
}

export interface DNS01ProviderLinks {
  api?: string
}

export interface DNS01Provider {
  name: string
  code: string
  links?: DNS01ProviderLinks
  /** Credential form, the only description of the values the provider takes. */
  form: DNSProviderForm
  propagation_timeout_seconds?: number
  polling_interval_seconds?: number
}

export interface PluginManifestDNS01 {
  providers: DNS01Provider[]
}

export interface PluginManifestHTTP {
  /** "unix" (reverse proxy to a socket) or "rpc" (http.handle fallback). */
  listen: string
}

/** Mirrors internal/plugin/protocol/manifest.go. */
/** Field types of a capability entry form. Absent means text. */
export type ConfigurationFieldType = 'text' | 'textarea' | 'number' | 'bool'

/**
 * One field of the form a plugin declares for a notify channel, a probe
 * kind, a storage backend, a deploy target, a blocklist source or a
 * discovery provider. Every value travels as a string: numbers in decimal,
 * booleans as "true" or "false".
 */
export interface ConfigurationField {
  key: string
  type?: ConfigurationFieldType
  display_name: string
  help_text?: string
  required?: boolean
  /** A credential: rendered as a masked input. */
  secret?: boolean
}

export interface ConfigurationSchema {
  fields?: ConfigurationField[]
}

export interface NotifyChannel {
  code: string
  name: string
  configuration?: ConfigurationSchema
}

export interface PluginManifestNotify {
  channels: NotifyChannel[]
}

export interface ProbeKind {
  code: string
  name: string
  configuration?: ConfigurationSchema
}

export interface PluginManifestProbe {
  kinds: ProbeKind[]
}

export interface MCPTool {
  name: string
  description: string
  input_schema?: Record<string, unknown>
}

export interface PluginManifestMCP {
  tools: MCPTool[]
}

export interface ManifestStorageBackend {
  code: string
  name: string
  configuration?: ConfigurationSchema
}

export interface PluginManifestStorage {
  backends: ManifestStorageBackend[]
}

export interface ManifestDeployTarget {
  code: string
  name: string
  configuration?: ConfigurationSchema
}

export interface PluginManifestDeploy {
  targets: ManifestDeployTarget[]
}

export interface ManifestBlocklistSource {
  code: string
  name: string
  configuration?: ConfigurationSchema
  /** Default refresh interval of a source of this kind, 0 means 3600. */
  refresh_seconds?: number
}

export interface PluginManifestBlocklist {
  sources: ManifestBlocklistSource[]
}

export interface ManifestDiscoveryProvider {
  code: string
  name: string
  configuration?: ConfigurationSchema
}

export interface PluginManifestDiscovery {
  providers: ManifestDiscoveryProvider[]
}

/** Line formats a log.sink plugin may ask for. */
export type LogSinkFormat = 'combined' | 'raw'

export interface PluginManifestLogSink {
  /** Most entries of one stream, 0 means 256. */
  batch_size?: number
  /** Longest time a stream stays open in milliseconds, 0 means 500. */
  flush_interval_ms?: number
  /** Formats the plugin wants, empty means every line. */
  formats?: LogSinkFormat[]
}

/** Translation of the display fields of a manifest into one language. */
export interface PluginManifestI18n {
  name?: string
  description?: string
}

export interface PluginManifest {
  id: string
  name: string
  version: string
  description?: string
  /** Locale code to the translated name and description, see spec MAN-40. */
  i18n?: Record<string, PluginManifestI18n>
  homepage_url?: string
  icon_path?: string
  api_version: number
  min_nginx_ui_version?: string
  server?: PluginManifestServer
  webapp?: PluginManifestWebapp
  content?: PluginManifestContent
  capabilities?: string[]
  permissions?: string[]
  requires?: PluginRequirement[]
  requires_capabilities?: string[]
  events?: string[]
  cron?: PluginManifestCron[]
  network_hosts?: string[]
  dns01?: PluginManifestDNS01
  http?: PluginManifestHTTP
  settings_schema?: SettingsSchema | null
  notify?: PluginManifestNotify
  probe?: PluginManifestProbe
  mcp?: PluginManifestMCP
  storage?: PluginManifestStorage
  deploy?: PluginManifestDeploy
  blocklist?: PluginManifestBlocklist
  discovery?: PluginManifestDiscovery
  log_sink?: PluginManifestLogSink
}

export interface PluginInfo {
  id: string
  name: string
  version: string
  description?: string
  homepage_url?: string
  icon_url?: string
  api_version: number
  min_nginx_ui_version?: string
  capabilities: string[]
  permissions: string[]
  requires: PluginRequirement[]
  requires_capabilities: string[]
  /** Addresses the plugin may reach, absent or empty when it names none. */
  network_hosts?: string[]
  has_server: boolean
  has_webapp: boolean
  lifecycle: PluginLifecycle
  status: PluginStatus
  enabled: boolean
  last_error?: string
  settings_schema?: SettingsSchema | null
  sync_policy: PluginSyncPolicy
  sync_node_ids: number[]
  sync_settings: boolean
  updated_at?: string
  /** Transport of capability calls, absent while no process is running. */
  transport?: PluginTransport
  dropped_events?: number
  /** Access log lines a log.sink plugin accepted since the host started. */
  streamed_log_entries?: number
  /** Access log lines a log.sink plugin received and discarded. */
  rejected_log_entries?: number
  /** Access log lines the host dropped before they reached the plugin. */
  dropped_log_entries?: number
  /** Limits of the plugin process, absent for a plugin without one. */
  resources?: PluginResources
  /** Locale code to translated name, `name` is the fallback. */
  name_i18n?: Record<string, string>
  /** Locale code to translated description, `description` is the fallback. */
  description_i18n?: Record<string, string>
  /** Trust level of the installed package, absent on an older node. */
  trust?: PluginTrust
  /** Signer id in 16 hex characters, empty or absent when unsigned. */
  signer?: string
}

export interface PluginResources {
  /** Memory limit in MiB, 0 when unlimited. */
  memory_limit_mb: number
  /** CPU limit in percent of one core, 0 when unlimited. */
  cpu_percent: number
  /** Whether the running process is confined to the limits. */
  enforced: boolean
}

export interface PluginInspect {
  manifest: PluginManifest
  permissions: string[]
  /** True when an upgrade asks for permissions the approved set does not cover. */
  permissions_changed: boolean
  installed_version?: string
  requires_missing: PluginRequirement[]
  /** "<goos>-<goarch>" builds the package ships, "any" when it runs everywhere. */
  platforms?: string[]
  /** "<goos>-<goarch>" of this node. */
  host_platform?: string
  /** Whether the package runs on this node. */
  platform_supported?: boolean
  /** Locale code to translated name, taken from `manifest.i18n`. */
  name_i18n?: Record<string, string>
  /** Locale code to translated description, taken from `manifest.i18n`. */
  description_i18n?: Record<string, string>
  /**
   * Id of the inspected package the node keeps for a while, so the install
   * does not upload it again. Absent on a node that does not keep uploads.
   */
  upload_id?: string
  /** Trust level derived from the package, absent on an older node. */
  trust?: PluginTrust
  /** Signer id in 16 hex characters, empty or absent when unsigned. */
  signer?: string
}

/** What an install reads the package from: the file or an inspected upload. */
export type PluginInstallSource = { file: File } | { uploadId: string }

export interface PluginSettingsResponse {
  schema: SettingsSchema | null
  values: Record<string, unknown>
}

/** Kind of a thing that depends on a plugin. */
export type PluginUsageKind = 'certificate'

export interface PluginUsageItem {
  kind: PluginUsageKind | string
  id: string
  name: string
}

/** What stops working on this node while a plugin is off. */
export interface PluginUsage {
  /** The first items, `total` counts all of them. */
  items: PluginUsageItem[]
  total: number
}

export interface PluginLogLine {
  time: string
  line: string
}

export interface PluginLogsResponse {
  lines: PluginLogLine[]
}

export interface WebappEntryPage {
  path: string
  title: Record<string, string>
  icon?: string
  file: string
}

export interface WebappEntry {
  id: string
  version: string
  bundle_url: string
  style_url?: string
  shared?: Record<string, string>
  pages?: WebappEntryPage[]
}

export interface PluginSpec {
  api_versions: number[]
  webapp_api_version: number
  capabilities: string[]
  transports: string[]
  /** "<goos>-<goarch>" a package must cover to run on this node. */
  platform?: string
}

/**
 * Picks the value for a language: the exact locale, then its base language,
 * then English, then the fallback, then any other translation.
 */
export function localizedText(values: Record<string, string> | undefined, language: string, fallback = ''): string {
  const base = language.split(/[-_]/)[0]
  for (const candidate of [language, base, 'en']) {
    const value = values?.[candidate]
    if (value)
      return value
  }
  if (fallback)
    return fallback

  return Object.values(values ?? {}).find(Boolean) ?? ''
}

/**
 * The display fields of an installed plugin (`PluginInfo`) or of a manifest
 * (`PluginManifest`), whichever carries the translations.
 */
export interface LocalizablePlugin {
  name: string
  description?: string
  name_i18n?: Record<string, string>
  description_i18n?: Record<string, string>
  i18n?: Record<string, PluginManifestI18n>
}

type LocalizedField = 'name' | 'description'

/** Non-empty translations of one field, from either shape. */
function translationsOf(plugin: LocalizablePlugin, field: LocalizedField): Record<string, string> {
  const values: Record<string, string> = {}
  for (const [locale, translated] of Object.entries(plugin.i18n ?? {})) {
    const value = translated?.[field]
    if (value)
      values[locale] = value
  }

  const flat = field === 'name' ? plugin.name_i18n : plugin.description_i18n
  for (const [locale, value] of Object.entries(flat ?? {})) {
    if (value)
      values[locale] = value
  }
  return values
}

/** Display name of a plugin in the given language, the manifest name as fallback. */
export function localizedPluginName(plugin: LocalizablePlugin, language: string): string {
  return localizedText(translationsOf(plugin, 'name'), language, plugin.name)
}

/** Description of a plugin in the given language, the manifest description as fallback. */
export function localizedPluginDescription(plugin: LocalizablePlugin, language: string): string {
  return localizedText(translationsOf(plugin, 'description'), language, plugin.description ?? '')
}

const multipartHeaders = {
  'Content-Type': 'multipart/form-data;charset=UTF-8',
}

function pluginPath(id: string, suffix = '') {
  return `/plugins/${encodeURIComponent(id)}${suffix}`
}

const plugin = {
  getList(): Promise<PluginInfo[]> {
    return http.get('/plugins')
  },

  /** Reads the manifest of an uploaded bundle without installing it. */
  inspect(file: File): Promise<PluginInspect> {
    const formData = new FormData()
    formData.append('file', file)

    return http.post('/plugins/inspect', formData, { headers: multipartHeaders })
  },

  /** Installs a new plugin or upgrades an existing one from the same bundle. */
  install(source: PluginInstallSource, enable = true, config?: HttpConfig): Promise<PluginInfo> {
    const formData = new FormData()
    if ('uploadId' in source)
      formData.append('upload_id', source.uploadId)
    else
      formData.append('file', source.file)
    formData.append('enable', enable ? 'true' : 'false')

    return http.post('/plugins', formData, { ...config, headers: multipartHeaders })
  },

  uninstall(id: string): Promise<unknown> {
    return http.delete(pluginPath(id))
  },

  enable(id: string, approvePermissions?: boolean): Promise<PluginInfo> {
    return http.post(pluginPath(id, '/enable'), { approve_permissions: approvePermissions })
  },

  disable(id: string): Promise<PluginInfo> {
    return http.post(pluginPath(id, '/disable'))
  },

  getSettings(id: string): Promise<PluginSettingsResponse> {
    return http.get(pluginPath(id, '/settings'))
  },

  saveSettings(id: string, values: Record<string, unknown>): Promise<PluginSettingsResponse> {
    return http.post(pluginPath(id, '/settings'), { settings: values })
  },

  getUsage(id: string): Promise<PluginUsage> {
    return http.get(pluginPath(id, '/usage'))
  },

  getLogs(id: string, lines = 500): Promise<PluginLogsResponse> {
    return http.get(pluginPath(id, '/logs'), { params: { lines } })
  },

  /** Browser bundles of every enabled plugin that ships a webapp. */
  getWebapp(): Promise<WebappEntry[]> {
    return http.get('/plugins/webapp')
  },

  getSpec(): Promise<PluginSpec> {
    return http.get('/plugins/spec')
  },
}

export default plugin
