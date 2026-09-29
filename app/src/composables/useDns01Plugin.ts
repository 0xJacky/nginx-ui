import type { ChallengeMethod } from '@/api/auto_cert'
import type { PluginInfo } from '@/api/plugin'
import auto_cert, { AutoCertChallengeMethod } from '@/api/auto_cert'
import pluginApi from '@/api/plugin'
import { getErrorMessage } from '@/lib/http'
import { usePluginLoader } from '@/plugin'
import { useGlobalApp } from './useGlobalApp'

/** The official plugin that contributes the DNS-01 challenge. */
export const DNS01_PLUGIN_ID = 'com.nginxui.dns01'

/**
 * - `unknown`: still loading, or the lookup failed; show nothing.
 * - `available`: DNS-01 can be used.
 * - `disabled`: the plugin is installed but turned off.
 * - `missing`: no plugin providing DNS-01 is installed.
 * - `broken`: the plugin is turned on but does not provide DNS-01 right now.
 */
export type Dns01PluginState = 'unknown' | 'available' | 'disabled' | 'missing' | 'broken'

const challengeMethods = ref<ChallengeMethod[]>([])
const installedPlugins = ref<PluginInfo[]>([])
const loaded = ref(false)
const loading = ref(false)
const enabling = ref(false)
let lastLoadedAt = 0
let pending: Promise<void> | undefined

// Several forms on one page ask at once, so answers are shared for a moment.
const REUSE_WINDOW_MS = 3000

async function fetchState() {
  loading.value = true
  try {
    const [methods, plugins] = await Promise.all([
      auto_cert.get_challenge_methods(),
      pluginApi.getList().catch(() => [] as PluginInfo[]),
    ])
    challengeMethods.value = methods
    installedPlugins.value = plugins
    loaded.value = true
    lastLoadedAt = Date.now()
  }
  catch {
    // Keep the previous answer; without one the hints stay hidden.
  }
  finally {
    loading.value = false
  }
}

function refresh(force = false) {
  if (!force && loaded.value && Date.now() - lastLoadedAt < REUSE_WINDOW_MS)
    return Promise.resolve()
  if (pending)
    return pending

  pending = fetchState().finally(() => {
    pending = undefined
  })
  return pending
}

const dns01Method = computed(() => challengeMethods.value.find(method => method.code === AutoCertChallengeMethod.dns01))

/** Id of the plugin to install or enable for DNS-01. */
const pluginId = computed(() => dns01Method.value?.plugin_id || DNS01_PLUGIN_ID)

const plugin = computed(() => installedPlugins.value.find(item => item.id === pluginId.value))

const state = computed<Dns01PluginState>(() => {
  if (!loaded.value)
    return 'unknown'
  if (dns01Method.value)
    return 'available'
  if (!plugin.value)
    return 'missing'
  if (!plugin.value.enabled || plugin.value.status === 'needs_approval')
    return 'disabled'
  return 'broken'
})

/**
 * Availability of the DNS-01 challenge and the actions that bring it back.
 * The state is shared by every caller, so enabling the plugin from one form
 * updates the hints everywhere on the page.
 */
export function useDns01Plugin() {
  const router = useRouter()
  const { message } = useGlobalApp()
  const installOpen = ref(false)
  const pluginLoader = usePluginLoader()

  onMounted(() => {
    void refresh()
  })

  function goToPlugins() {
    router.push({ path: '/system/plugins', query: plugin.value ? { plugin: plugin.value.id } : undefined })
  }

  async function afterChange() {
    await refresh(true)
    try {
      await pluginLoader.loadNew()
    }
    catch {
      // The slot falls back to the built-in form until the next reload.
    }
  }

  async function enable() {
    const target = plugin.value
    if (!target)
      return

    // New permissions have to be reviewed on the plugins page.
    if (target.status === 'needs_approval') {
      goToPlugins()
      return
    }

    enabling.value = true
    try {
      await pluginApi.enable(target.id)
      message.success($gettext('Plugin enabled'))
      await afterChange()
    }
    catch (e) {
      message.error(getErrorMessage(e, $gettext('Failed to enable the plugin')))
      await refresh(true)
    }
    finally {
      enabling.value = false
    }
  }

  /** Opens the install confirmation rendered by `Dns01PluginNotice`. */
  function install() {
    installOpen.value = true
  }

  return {
    state,
    pluginId,
    plugin,
    loading,
    enabling,
    installOpen,
    isAvailable: computed(() => state.value === 'available' || state.value === 'unknown'),
    refresh,
    enable,
    install,
    onInstalled: afterChange,
    goToPlugins,
  }
}

export type Dns01Plugin = ReturnType<typeof useDns01Plugin>
