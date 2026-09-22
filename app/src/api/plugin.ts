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

export type SettingsFieldType = 'text' | 'bool' | 'number' | 'select' | 'secret' | 'textarea'

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

export interface DNS01ProviderConfig {
  credentials?: Record<string, string>
  additional?: Record<string, string>
}

export interface DNS01ProviderLinks {
  api?: string
  go_client?: string
}

export interface DNS01Provider {
  name: string
  code: string
  configuration?: DNS01ProviderConfig
  links?: DNS01ProviderLinks
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
/** Field types of a notify channel or probe kind form. Absent means text. */
export type ConfigurationFieldType = 'text' | 'textarea' | 'number' | 'bool'

/**
 * One field of the form a plugin declares for a notify channel or a probe
 * kind. Every value travels as a string: numbers in decimal, booleans as
 * "true" or "false".
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

export interface PluginManifest {
  id: string
  name: string
  version: string
  description?: string
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
}

export interface PluginSettingsResponse {
  schema: SettingsSchema | null
  values: Record<string, unknown>
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
  install(file: File, enable = true): Promise<PluginInfo> {
    const formData = new FormData()
    formData.append('file', file)
    formData.append('enable', enable ? 'true' : 'false')

    return http.post('/plugins', formData, { headers: multipartHeaders })
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
