import type { RouteRecordRaw } from 'vue-router'
import type { PluginInfo, PluginManifest, PluginManifestI18n, WebappEntry, WebappEntryPage } from '@/api/plugin'
import pluginApi from '@/api/plugin'
import gettext from '@/gettext'
import router, { NOT_FOUND_ROUTE_NAME } from '@/routes'
import { createBundleSource } from './bundles'
import { injectScript, scriptQueue, withVersion } from './chunks'
import { isLoopbackUrl } from './loopback'
import { createRegistry } from './registry'
import { removePluginRoutes, trackPluginRoute } from './routeRemovers'
import { satisfies } from './semver'
import { takePendingPlugin } from './shared'
import { usePluginStore } from './store'

const IFRAME_PAGE_COMPONENT = () => import('@/views/plugin/IframePage.vue')

/** Page the user is sent to when the plugin behind the open page goes away. */
const FALLBACK_PATH = '/system/plugins'

/** Initial load in flight, shared by every caller of `load()`. */
let initialLoad: Promise<void> | undefined

/**
 * Runs a bundle script and takes the definition it registered. Definitions are
 * remembered per version, so a plugin enabled again after `unload` runs its
 * setup again instead of evaluating the script a second time.
 */
const bundles = createBundleSource<WebappEntry>(entry =>
  // The bundle hands its definition over on a global, so no other script of
  // the page runs until it has been taken.
  scriptQueue.run(async () => {
    await injectScript(withVersion(entry.bundle_url, entry.version))
    return takePendingPlugin(entry.id)
  }))

/** Injects a stylesheet once and resolves as soon as it is applied. */
function injectStyle(url: string): Promise<void> {
  return new Promise(resolve => {
    if (document.querySelector(`link[data-nginx-ui-plugin-style="${url}"]`)) {
      resolve()
      return
    }

    const link = document.createElement('link')
    link.rel = 'stylesheet'
    link.href = url
    link.dataset.nginxUiPluginStyle = url
    // A missing stylesheet must not stop the bundle from loading.
    link.onload = () => resolve()
    link.onerror = () => resolve()
    document.head.appendChild(link)
  })
}

/** Compares the ranges a bundle was built against with the shared runtime. */
function checkSharedRuntime(entry: WebappEntry): string {
  const versions = window.NginxUI?.shared?.versions ?? {}

  for (const [name, range] of Object.entries(entry.shared ?? {})) {
    const current = versions[name]

    if (!current)
      return `${name} is not part of the shared runtime`

    if (!satisfies(current, range))
      return `${name}@${current} does not satisfy ${range}`
  }

  return ''
}

/** Rebuilds the manifest i18n block from the flat maps of the plugin list. */
function manifestI18nFromInfo(info: PluginInfo): Record<string, PluginManifestI18n> | undefined {
  const i18n: Record<string, PluginManifestI18n> = {}
  for (const [locale, name] of Object.entries(info.name_i18n ?? {}))
    i18n[locale] = { ...i18n[locale], name }
  for (const [locale, description] of Object.entries(info.description_i18n ?? {}))
    i18n[locale] = { ...i18n[locale], description }

  return Object.keys(i18n).length > 0 ? i18n : undefined
}

/** Fills in the manifest fields the plugin list already knows about. */
function manifestFromInfo(entry: WebappEntry, info?: PluginInfo): PluginManifest {
  return {
    id: entry.id,
    name: info?.name ?? entry.id,
    version: info?.version ?? entry.version,
    description: info?.description,
    i18n: info ? manifestI18nFromInfo(info) : undefined,
    homepage_url: info?.homepage_url,
    api_version: info?.api_version ?? 0,
    min_nginx_ui_version: info?.min_nginx_ui_version,
    capabilities: info?.capabilities ?? [],
    permissions: info?.permissions ?? [],
    requires: info?.requires ?? [],
    requires_capabilities: info?.requires_capabilities ?? [],
    settings_schema: info?.settings_schema ?? null,
    webapp: {
      bundle_path: entry.bundle_url,
      style_path: entry.style_url,
      chunks: entry.chunks,
      shared: entry.shared,
      pages: entry.pages,
    },
  }
}

function pageTitle(page: WebappEntryPage) {
  return page.title?.[gettext.current] ?? page.title?.en ?? page.path
}

/**
 * Registers one route per manifest page. The URL shape is
 * `plugins/{id}/pages/{path}` and the component is the host iframe wrapper.
 */
function registerIframePages(entry: WebappEntry) {
  const store = usePluginStore()

  for (const page of entry.pages ?? []) {
    const record: RouteRecordRaw = {
      path: `plugins/${entry.id}/pages/${page.path}`,
      name: `Plugin Page ${entry.id} ${page.path}`,
      component: IFRAME_PAGE_COMPONENT,
      props: { pluginId: entry.id, file: page.file },
      meta: {
        name: () => pageTitle(page),
        pluginId: entry.id,
      },
    }

    trackPluginRoute(entry.id, router.addRoute('Home', record))
    store.addRoute(record)
  }
}

/** Reads a dev plugin.json and turns it into a loadable webapp entry. */
async function fetchDevEntry(url: string): Promise<{ entry: WebappEntry, manifest: PluginManifest } | null> {
  const response = await fetch(url, { cache: 'no-store' })
  if (!response.ok)
    throw new Error(`HTTP ${response.status}`)

  const manifest = await response.json() as PluginManifest
  const webapp = manifest.webapp

  if (!manifest.id || (!webapp?.bundle_path && !webapp?.pages?.length))
    throw new Error('plugin.json declares neither webapp.bundle_path nor webapp.pages')

  // Bundle and stylesheet live next to the manifest that points at them.
  const base = new URL(url, window.location.href)
  const bundleUrl = webapp.bundle_path ? new URL(webapp.bundle_path, base).toString() : ''
  const styleUrl = webapp.style_path ? new URL(webapp.style_path, base).toString() : undefined

  const chunkUrls: Record<string, string> = {}
  for (const [name, file] of Object.entries(webapp.chunks ?? {}))
    chunkUrls[name] = new URL(file, base).toString()

  // An absolute path in the manifest must not lead off the loopback host either.
  for (const assetUrl of [bundleUrl, styleUrl, ...Object.values(chunkUrls)]) {
    if (assetUrl && !isLoopbackUrl(assetUrl))
      throw new Error(`${assetUrl} is not a localhost address`)
  }

  return {
    manifest,
    entry: {
      id: manifest.id,
      version: manifest.version ?? String(Date.now()),
      bundle_url: bundleUrl,
      style_url: styleUrl,
      chunks: Object.keys(chunkUrls).length > 0 ? chunkUrls : undefined,
      shared: webapp.shared,
      pages: webapp.pages,
    },
  }
}

/**
 * A plugin URL opened before its route existed was claimed by the catch-all
 * route. Moves to the real route once the loader added it.
 */
async function leaveNotFound() {
  const current = router.currentRoute.value
  if (current.name !== NOT_FOUND_ROUTE_NAME || router.resolve(current.fullPath).name === NOT_FOUND_ROUTE_NAME)
    return

  try {
    await router.replace(current.fullPath)
  }
  catch (error) {
    console.error(`[plugin] could not open ${current.fullPath}`, error)
  }
}

export function usePluginLoader() {
  const store = usePluginStore()

  async function loadEntry(entry: WebappEntry, manifest: PluginManifest) {
    const incompatibility = checkSharedRuntime(entry)
    if (incompatibility) {
      console.warn(`[plugin] ${entry.id}: skipped, ${incompatibility}`)
      store.setLoadState(entry.id, 'incompatible')
      return
    }

    // Manifest pages are plain static files, so they work without a bundle and
    // survive a bundle that fails to set itself up.
    registerIframePages(entry)

    if (!entry.bundle_url) {
      store.setLoadState(entry.id, 'loaded')
      return
    }

    if (entry.style_url)
      await injectStyle(withVersion(entry.style_url, entry.version))

    const definition = await bundles.acquire(entry)
    if (!definition) {
      console.warn(`[plugin] ${entry.id}: bundle did not call registerPlugin('${entry.id}', ...)`)
      store.setLoadState(entry.id, 'failed')
      return
    }

    await definition.setup(createRegistry(entry.id, manifest, entry))
    store.setLoadState(entry.id, 'loaded')
  }

  /** One plugin failing must never take the rest of the application down. */
  async function loadEntrySafely(entry: WebappEntry, manifest: PluginManifest) {
    try {
      await loadEntry(entry, manifest)
    }
    catch (error) {
      console.error(`[plugin] ${entry.id}: failed to load`, error)
      store.setLoadState(entry.id, 'failed')
    }
  }

  async function loadDevPlugin() {
    const url = store.devPluginUrl.trim()
    if (!url)
      return

    // The URL is persisted, so a value stored before the check existed or
    // written by hand must not load a remote script.
    if (!isLoopbackUrl(url)) {
      console.warn(`[plugin] dev plugin URL ${url} ignored, only localhost addresses are accepted`)
      return
    }

    try {
      const dev = await fetchDevEntry(url)
      if (dev)
        await loadEntrySafely(dev.entry, dev.manifest)
    }
    catch (error) {
      console.error(`[plugin] dev plugin at ${url} could not be loaded`, error)
    }
  }

  async function loadAll() {
    if (store.loading)
      return

    store.loading = true

    try {
      const entries = await pluginApi.getWebapp().catch(error => {
        console.error('[plugin] failed to list plugin bundles', error)
        return [] as WebappEntry[]
      })

      store.entries = entries

      if (entries.length > 0) {
        const list = await pluginApi.getList().catch(() => [] as PluginInfo[])
        const infoById = new Map(list.map(info => [info.id, info]))

        // Sequential on purpose: a bundle registers itself on a global, so two
        // scripts must not be in flight at the same time.
        for (const entry of entries)
          await loadEntrySafely(entry, manifestFromInfo(entry, infoById.get(entry.id)))
      }

      await loadDevPlugin()
    }
    finally {
      store.loading = false
      store.ready = true
    }
  }

  /**
   * Loads every enabled webapp bundle, then the dev plugin if one is set.
   * Runs once per page load; the sidebar and the slots render as soon as
   * `ready` flips. A call made while the load is in flight waits for it, and
   * the returned promise never rejects.
   */
  function load(): Promise<void> {
    if (store.ready)
      return Promise.resolve()

    initialLoad ??= loadAll().finally(() => {
      initialLoad = undefined
    })

    return initialLoad
  }

  /**
   * Loads bundles of plugins enabled after the page was loaded. Entries that
   * already went through the loader are left alone. A plugin that was
   * unloaded reuses its remembered definition when the version is the same,
   * because a bundle cannot be evaluated twice on the same page.
   */
  async function loadNew() {
    if (store.loading)
      return

    store.loading = true

    try {
      const entries = await pluginApi.getWebapp().catch(() => [] as WebappEntry[])
      const pending = entries.filter(entry => !(entry.id in store.loaded))
      if (pending.length === 0)
        return

      store.entries = entries

      const list = await pluginApi.getList().catch(() => [] as PluginInfo[])
      const infoById = new Map(list.map(info => [info.id, info]))

      for (const entry of pending)
        await loadEntrySafely(entry, manifestFromInfo(entry, infoById.get(entry.id)))

      await leaveNotFound()
    }
    finally {
      store.loading = false
    }
  }

  /**
   * Takes the contributions of a plugin out of the running page: slots,
   * routes and the settings panel, after the plugin's own teardown. The definition stays remembered unless
   * `forget` is set, so the plugin can be turned on again without a reload.
   */
  async function unload(pluginId: string, options: { forget?: boolean } = {}) {
    // Leave a page that is about to disappear before its route is removed.
    const current = router.currentRoute.value
    if (current.matched.some(record => record.meta?.pluginId === pluginId)) {
      try {
        await router.replace(FALLBACK_PATH)
      }
      catch (error) {
        console.error(`[plugin] ${pluginId}: could not leave its page`, error)
      }
    }

    // The plugin cleans up what it started outside its components, such as
    // timers or connections. Its failure must not keep the rest in place.
    if (store.loaded[pluginId] === 'loaded') {
      try {
        bundles.peek(pluginId)?.teardown?.()
      }
      catch (error) {
        console.error(`[plugin] ${pluginId}: teardown failed`, error)
      }
    }

    removePluginRoutes(pluginId)
    store.removePlugin(pluginId)

    if (options.forget)
      bundles.forget(pluginId)
  }

  /**
   * Unloads the bundles of plugins that are installed but turned off, for
   * example after an install turned off a plugin that cannot run beside it.
   */
  async function unloadDisabled(plugins: Pick<PluginInfo, 'id' | 'enabled'>[]) {
    for (const plugin of plugins) {
      if (!plugin.enabled && plugin.id in store.loaded)
        await unload(plugin.id)
    }
  }

  return { load, loadNew, unload, unloadDisabled }
}
