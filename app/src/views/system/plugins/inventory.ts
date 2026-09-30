import type { ComputedRef, InjectionKey, MaybeRefOrGetter, Ref } from 'vue'
import type { PluginInfo } from '@/api/plugin'
import type { CatalogEntry, PluginUpdateInfo } from '@/api/plugin_marketplace'
import { useIntervalFn } from '@vueuse/core'
import nodeApi from '@/api/node'
import pluginApi from '@/api/plugin'
import { getMarketplaceList, getPluginUpdates } from '@/api/plugin_marketplace'
import { getErrorMessage } from '@/lib/http'

export interface PluginInventory {
  plugins: Ref<PluginInfo[]>
  updates: Ref<PluginUpdateInfo[]>
  /** The unfiltered marketplace catalog, empty while it is unavailable. */
  catalog: Ref<CatalogEntry[]>
  /** True once this instance has at least one enabled child node. */
  hasNodes: Ref<boolean>
  loading: Ref<boolean>
  error: Ref<string>
  /** Reloads the installed list. A silent reload keeps the current view in place. */
  reload: (silent?: boolean) => Promise<void>
  reloadUpdates: () => Promise<void>
  reloadCatalog: () => Promise<void>
}

const inventoryKey: InjectionKey<PluginInventory> = Symbol('plugin-inventory')

/**
 * One copy of the installed list shared by the page header and every tab, so
 * the counters and the cards never disagree.
 */
export function providePluginInventory(): PluginInventory {
  const plugins = ref<PluginInfo[]>([])
  const updates = ref<PluginUpdateInfo[]>([])
  const catalog = ref<CatalogEntry[]>([])
  const hasNodes = ref(false)
  const loading = ref(false)
  const error = ref('')

  async function reload(silent = false) {
    if (!silent)
      loading.value = true

    try {
      plugins.value = await pluginApi.getList()
      error.value = ''
    }
    catch (e) {
      error.value = getErrorMessage(e, $gettext('Failed to load the plugin list'))
    }
    finally {
      loading.value = false
    }
  }

  async function reloadUpdates() {
    try {
      updates.value = await getPluginUpdates()
    }
    catch {
      // The counter is a convenience, a failed probe must not break the page.
      updates.value = []
    }
  }

  async function reloadCatalog() {
    try {
      catalog.value = (await getMarketplaceList()).plugins
    }
    catch {
      // Only the offers to install a more trusted version depend on it.
      catalog.value = []
    }
  }

  async function loadNodes() {
    try {
      const { data } = await nodeApi.getList({ enabled: true })
      hasNodes.value = data.length > 0
    }
    catch {
      hasNodes.value = false
    }
  }

  // A plugin that is still starting settles within seconds, so poll until
  // nothing is in flight instead of asking the user to refresh.
  const hasTransientState = computed(() => plugins.value.some(item => item.status === 'starting'))
  const { pause, resume } = useIntervalFn(() => reload(true), 5000, { immediate: false })

  watch(hasTransientState, value => {
    if (value)
      resume()
    else
      pause()
  })

  onMounted(() => {
    void reload()
    void reloadUpdates()
    void reloadCatalog()
    void loadNodes()
  })
  onUnmounted(pause)

  const inventory: PluginInventory = {
    plugins,
    updates,
    catalog,
    hasNodes,
    loading,
    error,
    reload,
    reloadUpdates,
    reloadCatalog,
  }
  provide(inventoryKey, inventory)
  return inventory
}

export function usePluginInventory(): PluginInventory {
  const inventory = inject(inventoryKey)
  if (!inventory)
    throw new Error('usePluginInventory must be called inside the plugins page')

  return inventory
}

/**
 * Every installed plugin. Reads the inventory of the plugins page, or fetches
 * the list when the caller lives outside of it, once `active` turns true.
 */
export function useInstalledPlugins(active: MaybeRefOrGetter<boolean> = true): ComputedRef<PluginInfo[]> {
  const inventory = inject(inventoryKey, undefined)
  const fetched = shallowRef<PluginInfo[]>([])

  if (!inventory) {
    watch(() => toValue(active), async isActive => {
      if (!isActive)
        return

      try {
        fetched.value = await pluginApi.getList()
      }
      catch {
        // Only hints depend on it, so a failed lookup shows none.
      }
    }, { immediate: true })
  }

  return computed(() => (inventory ? inventory.plugins.value : fetched.value))
}

/**
 * The installed plugin with the given id. Reads the inventory of the plugins
 * page, or fetches the plugin list when the caller lives outside of it. An
 * empty id skips the lookup.
 */
export function useInstalledPlugin(pluginId: MaybeRefOrGetter<string | undefined>): ComputedRef<PluginInfo | undefined> {
  const inventory = inject(inventoryKey, undefined)
  const fetched = shallowRef<PluginInfo>()

  if (!inventory) {
    watch(() => toValue(pluginId), async id => {
      fetched.value = undefined
      if (!id)
        return

      try {
        const list = await pluginApi.getList()
        // Drop a late answer for an id that is no longer asked for.
        if (toValue(pluginId) === id)
          fetched.value = list.find(item => item.id === id)
      }
      catch {
        // Only hints depend on it, so a failed lookup shows none.
      }
    }, { immediate: true })
  }

  return computed(() => {
    const id = toValue(pluginId)
    if (!id)
      return undefined
    return inventory
      ? inventory.plugins.value.find(item => item.id === id)
      : fetched.value
  })
}
