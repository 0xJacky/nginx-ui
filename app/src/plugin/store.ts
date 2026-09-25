import type { Component } from 'vue'
import type { RouteRecordRaw } from 'vue-router'
import type { PluginLoadState, RegisterSlotOptions, SlotContext, SlotName, SlotRegistration } from './types'
import type { WebappEntry } from '@/api/plugin'

/**
 * Everything plugin bundles contributed to the running application.
 *
 * Components and route records are stored with `markRaw` / `shallowRef` so Vue
 * never wraps a component definition in a reactive proxy.
 */
export const usePluginStore = defineStore('plugin', () => {
  /** True once the loader finished, successfully or not. */
  const ready = ref(false)
  const loading = ref(false)
  const entries = shallowRef<WebappEntry[]>([])
  const loaded = ref<Record<string, PluginLoadState>>({})
  const routes = shallowRef<RouteRecordRaw[]>([])
  const slots = shallowRef<Record<string, SlotRegistration[]>>({})
  const settingsPanels = shallowRef<Record<string, Component>>({})
  /**
   * Address of a plugin.json served on a loopback host, used to load a plugin
   * straight from its build output while developing it. The loader ignores
   * any other address.
   */
  const devPluginUrl = ref('')

  function addRoute(route: RouteRecordRaw) {
    routes.value = [...routes.value, route]
  }

  function addSlot(name: SlotName, pluginId: string, component: Component, options: RegisterSlotOptions = {}) {
    const key = String(name)
    const registration: SlotRegistration = {
      pluginId,
      component: markRaw(component),
      order: options.order ?? 0,
      when: options.when,
    }

    slots.value = {
      ...slots.value,
      [key]: [...(slots.value[key] ?? []), registration],
    }
  }

  /** Registrations of one slot that apply to `ctx`, in render order. */
  function slotComponents(name: SlotName, ctx: SlotContext = {}): SlotRegistration[] {
    const registrations = slots.value[String(name)] ?? []

    return registrations
      .filter(registration => !registration.when || registration.when(ctx))
      .sort((a, b) => a.order - b.order)
  }

  function setSettingsPanel(pluginId: string, component: Component) {
    settingsPanels.value = {
      ...settingsPanels.value,
      [pluginId]: markRaw(component),
    }
  }

  function setLoadState(pluginId: string, state: PluginLoadState) {
    loaded.value[pluginId] = state
  }

  return {
    ready,
    loading,
    entries,
    loaded,
    routes,
    slots,
    settingsPanels,
    devPluginUrl,
    addRoute,
    addSlot,
    slotComponents,
    setSettingsPanel,
    setLoadState,
  }
}, {
  persist: {
    pick: ['devPluginUrl'],
  },
})
