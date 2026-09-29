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
}

/** Slots the host renders today. Any other string is accepted but ignored. */
export type KnownSlotName
  = | `certificate.challenge.form:${string}`
    | `dns.credential.form:${string}`
    | `dns.credential.hint:${string}`
    | 'certificate.issue.footer'
    | `plugin.settings:${string}`
    | 'sidebar.footer'

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

export interface RegisterSlotOptions {
  /** Lower values render first. */
  order?: number
  /** Return false to skip rendering for a given context. */
  when?: (ctx: SlotContext) => boolean
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
}

export type PluginLoadState = 'loaded' | 'failed' | 'incompatible'

declare global {
  interface Window {
    NginxUI: NginxUIGlobal
  }
}
