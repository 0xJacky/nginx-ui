import type { http } from '@uozi-admin/request'
import type { AxiosInstance } from 'axios'
import type { Component } from 'vue'
import type { RouteRecordRaw } from 'vue-router'
import type { PluginManifest } from '@/api/plugin'
import type gettext from '@/gettext'

/**
 * Module instances the host shares with plugin bundles. A bundle must never
 * ship its own copy of these: reactivity only works inside one Vue instance.
 */
export interface SharedRuntime {
  vue: typeof import('vue')
  vueRouter: typeof import('vue-router')
  pinia: typeof import('pinia')
  antdvNext: typeof import('antdv-next')
  antdvIcons: typeof import('@antdv-next/icons')
  vueuse: typeof import('@vueuse/core')
  gettext: typeof gettext
  http: typeof http
  /** Resolved versions of the shared libraries, keyed by package name. */
  versions: Record<string, string>
  /** Host dialogs a plugin may open. Older hosts do not have it, so check before use. */
  ui: SharedUI
}

/** A DNS credential created from `SharedUI.openDnsCredentialEditor`. */
export interface SharedDnsCredential {
  id: number
  name: string
  code: string
  provider?: string
  provider_code?: string
}

export interface SharedUI {
  /**
   * Opens the host DNS credential form in a modal on top of the current page.
   * Resolves with the created credential, or undefined when it is cancelled.
   */
  openDnsCredentialEditor: () => Promise<SharedDnsCredential | undefined>
}

export interface NginxUIGlobal {
  /** Host application version. */
  version: string
  shared: SharedRuntime
  registerPlugin: (id: string, definition: NginxUIPlugin) => void
  /** Called by a chunk file while its script executes. Absent on hosts without chunk support. */
  registerChunk?: (pluginId: string, name: string, exports: Record<string, unknown>) => void
}

/** Slots the host renders today. Any other string is accepted but ignored. */
export type KnownSlotName
  = | `certificate.challenge.form:${string}`
    | `dns.credential.form:${string}`
    | `dns.credential.hint:${string}`
    | 'certificate.issue.footer'
    | `plugin.settings:${string}`
    | 'sidebar.footer'
    | `nginx_log.view:${string}`
    | 'nginx_log.list.toolbar'
    | `nginx_log.list.column:${string}`
    | 'nginx_log.list.row.actions'
    | 'site.log.actions'

export type SlotName = KnownSlotName | (string & Record<never, never>)

/** Payload the host passes to the components mounted in a slot. */
export interface SlotContext {
  [key: string]: unknown
}

export interface RegisterRouteOptions {
  /**
   * Name of an existing top level sidebar entry (for example `System`) the new
   * item should be nested under. The route itself always lives under the main
   * layout; this only affects where the sidebar shows it.
   */
  parent?: string
  /** Lower values sort first inside the sidebar group. */
  order?: number
}

/**
 * The list row of a log file, as the log list shows it. A host may add fields,
 * a plugin must ignore the ones it does not know.
 */
export interface NginxLogRow {
  path: string
  type: 'access' | 'error' | string
  name: string
  config_file: string
  [key: string]: unknown
}

/** One choice of a filterable log list column. */
export interface SlotColumnFilter {
  /** English source string, translated by the host. */
  label: string
  /** Stable identifier of the choice. */
  value: string
  match: (row: NginxLogRow) => boolean
}

/** Value a sortable column reads from a row. Null and undefined sort last. */
export type SlotSortValue = string | number | null | undefined

export interface RegisterSlotOptions {
  /** Lower values render first. */
  order?: number
  /** Return false to skip rendering for a given context. */
  when?: (ctx: SlotContext) => boolean
  /**
   * Display text of `nginx_log.view:{key}` and `nginx_log.list.column:{key}`,
   * an English source string the host translates.
   */
  label?: string
  /** Makes an `nginx_log.list.column:{key}` column sortable. */
  sortValue?: (row: NginxLogRow) => SlotSortValue
  /** Makes an `nginx_log.list.column:{key}` column filterable. */
  filters?: SlotColumnFilter[]
}

/** Read-only view of the host state a plugin is allowed to observe. */
export interface PluginHostState {
  readonly theme: 'light' | 'dark'
  readonly locale: string
  readonly nodeId: number
  readonly username: string
}

export interface PluginRegistry {
  /** Adds a child route under the main layout; the sidebar entry is derived from meta. */
  registerRoute: (route: RouteRecordRaw, options?: RegisterRouteOptions) => void
  /** Mounts a component into a host-defined extension slot. */
  registerSlot: (slot: SlotName, component: Component, options?: RegisterSlotOptions) => void
  /** Merges gettext messages for a locale; keys are the English source strings. */
  registerTranslations: (locale: string, messages: Record<string, string>) => void
  /** Replaces the schema-driven settings form with a custom component. */
  registerSettingsPanel: (component: Component) => void
  /**
   * Loads an on-demand chunk declared in `webapp.chunks` and resolves with the
   * exports the chunk handed to `registerChunk`. Absent on hosts without chunk
   * support, so check before use.
   */
  loadChunk: (name: string) => Promise<Record<string, unknown>>
  /** Client whose baseURL is ./api/plugins/{id}/http. */
  http: AxiosInstance
  /** The host API client, usable only with the `core_api` permission. */
  coreHttp: typeof http
  manifest: PluginManifest
  host: PluginHostState
}

export interface NginxUIPlugin {
  setup: (registry: PluginRegistry) => void | Promise<void>
  teardown?: () => void
}

export interface SlotRegistration {
  pluginId: string
  component: Component
  order: number
  when?: (ctx: SlotContext) => boolean
  label?: string
  sortValue?: (row: NginxLogRow) => SlotSortValue
  filters?: SlotColumnFilter[]
}

export type PluginLoadState = 'loaded' | 'failed' | 'incompatible'

declare global {
  interface Window {
    NginxUI: NginxUIGlobal
  }
}
