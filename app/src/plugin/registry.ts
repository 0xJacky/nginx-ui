import type { AxiosInstance } from 'axios'
import type { Component } from 'vue'
import type { RouteMeta, RouteRecordRaw } from 'vue-router'
import type {
  PluginHostState,
  PluginRegistry,
  RegisterRouteOptions,
  RegisterSlotOptions,
  SlotName,
} from './types'
import type { PluginManifest, WebappEntry } from '@/api/plugin'
import { http } from '@uozi-admin/request'
import axios from 'axios'
import { localizedPluginName } from '@/api/plugin'
import gettext from '@/gettext'
import { applyAuthHeaders } from '@/lib/http/interceptors'
import { useSettingsStore, useUserStore } from '@/pinia'
import router from '@/routes'
import { chunkLoader } from './chunks'
import { usePluginStore } from './store'

/** Route name plugin routes are attached to, i.e. the authenticated layout. */
const LAYOUT_ROUTE_NAME = 'Home'

/**
 * Client a plugin uses to reach its own backend.
 *
 * It does not go through the core response interceptor on purpose: a plugin
 * owns its payload shape, so it gets plain axios semantics and the host error
 * toasts stay out of the way. Authentication is shared with the core client.
 */
function createPluginHttp(pluginId: string): AxiosInstance {
  const instance = axios.create({
    baseURL: `./api/plugins/${encodeURIComponent(pluginId)}/http`,
  })

  instance.interceptors.request.use(applyAuthHeaders)

  return instance
}

/** Read-only projection of the host state, without handing over any store. */
function createHostState(): PluginHostState {
  const settings = useSettingsStore()
  const user = useUserStore()

  const state = reactive({
    theme: computed(() => (settings.theme === 'dark' ? 'dark' : 'light')),
    locale: computed(() => gettext.current),
    nodeId: computed(() => settings.node.id),
    username: computed(() => user.info?.name ?? ''),
  })

  return readonly(state) as PluginHostState
}

/**
 * Builds the object a plugin bundle receives in `setup()`.
 *
 * Everything a plugin registers is tagged with its id so the host can tell
 * contributions apart when one of them misbehaves.
 */
export function createRegistry(
  pluginId: string,
  manifest: PluginManifest,
  webapp?: Pick<WebappEntry, 'version' | 'chunks'>,
): PluginRegistry {
  const store = usePluginStore()
  const pluginHttp = createPluginHttp(pluginId)
  const host = createHostState()

  function registerRoute(route: RouteRecordRaw, options: RegisterRouteOptions = {}) {
    const meta: RouteMeta = {
      ...(route.meta ?? {}),
      name: route.meta?.name ?? (() => localizedPluginName(manifest, gettext.current) || pluginId),
      pluginId,
      pluginParent: options.parent,
      pluginOrder: options.order ?? 0,
    }

    const record = { ...route, meta } as RouteRecordRaw

    router.addRoute(LAYOUT_ROUTE_NAME, record)
    store.addRoute(record)
  }

  function registerSlot(slot: SlotName, component: Component, options: RegisterSlotOptions = {}) {
    store.addSlot(slot, pluginId, component, options)
  }

  function registerTranslations(locale: string, messages: Record<string, string>) {
    if (!locale || !messages)
      return

    // Replacing the whole language entry keeps the assignment reactive, which
    // a nested mutation on the gettext translations object would not be.
    gettext.translations[locale] = {
      ...(gettext.translations[locale] ?? {}),
      ...messages,
    }
  }

  function registerSettingsPanel(component: Component) {
    store.setSettingsPanel(pluginId, component)
  }

  function loadChunk(name: string) {
    return chunkLoader.loadChunk({
      pluginId,
      version: webapp?.version ?? manifest.version,
      chunks: webapp?.chunks,
    }, name)
  }

  return {
    registerRoute,
    registerSlot,
    registerTranslations,
    registerSettingsPanel,
    loadChunk,
    http: pluginHttp,
    coreHttp: http,
    manifest,
    host,
  }
}
