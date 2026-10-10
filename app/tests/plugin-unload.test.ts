import type { RouteRecordRaw } from 'vue-router'
import type { NginxUIPlugin } from '../src/plugin/types'
import { beforeEach, describe, expect, test } from 'bun:test'
import { createPinia, defineStore, setActivePinia } from 'pinia'
import { markRaw, ref, shallowRef } from 'vue'
import { createBundleSource } from '../src/plugin/bundles'
import { createChunkLoader } from '../src/plugin/chunks'
import { removePluginRoutes, trackPluginRoute } from '../src/plugin/routeRemovers'

Object.assign(globalThis, { defineStore, markRaw, ref, shallowRef })

const { usePluginStore } = await import('../src/plugin/store')

function route(pluginId: string, path: string): RouteRecordRaw {
  return { path, component: {}, meta: { pluginId } } as RouteRecordRaw
}

describe('plugin store removePlugin', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  test('drops slots, routes, settings panel and load state of one plugin only', () => {
    const store = usePluginStore()

    for (const id of ['a', 'b']) {
      store.addSlot('nginx_log.list.column:geo', id, {})
      store.addRoute(route(id, `/${id}`))
      store.setSettingsPanel(id, {})
      store.setLoadState(id, 'loaded')
    }
    store.addSlot('site.log.actions', 'a', {})
    store.entries = [
      { id: 'a', version: '1', bundle_url: 'a.js' },
      { id: 'b', version: '1', bundle_url: 'b.js' },
    ]

    store.removePlugin('a')

    expect(store.slots['nginx_log.list.column:geo'].map(item => item.pluginId)).toEqual(['b'])
    expect(store.slots['site.log.actions']).toBeUndefined()
    expect(store.routes.map(item => item.meta?.pluginId)).toEqual(['b'])
    expect(Object.keys(store.settingsPanels)).toEqual(['b'])
    expect(store.entries.map(item => item.id)).toEqual(['b'])
    expect(store.loaded).toEqual({ b: 'loaded' })
    expect(store.slotComponents('nginx_log.list.column:geo')).toHaveLength(1)
  })

  test('replaces the collections so watchers see the change', () => {
    const store = usePluginStore()
    store.addSlot('sidebar.footer', 'a', {})
    store.addRoute(route('a', '/a'))
    const slotsBefore = store.slots
    const routesBefore = store.routes

    store.removePlugin('a')

    expect(store.slots).not.toBe(slotsBefore)
    expect(store.routes).not.toBe(routesBefore)
  })
})

describe('plugin route removers', () => {
  test('calls every remover of the plugin once', () => {
    const calls: string[] = []
    trackPluginRoute('a', () => calls.push('a1'))
    trackPluginRoute('a', () => calls.push('a2'))
    trackPluginRoute('b', () => calls.push('b1'))

    removePluginRoutes('a')
    removePluginRoutes('a')

    expect(calls).toEqual(['a1', 'a2'])

    removePluginRoutes('b')
    expect(calls).toEqual(['a1', 'a2', 'b1'])
  })
})

describe('bundle source', () => {
  interface Entry {
    id: string
    version: string
  }

  function fixture() {
    const injected: string[] = []
    const definitions: NginxUIPlugin[] = []
    const source = createBundleSource<Entry>(async entry => {
      injected.push(`${entry.id}@${entry.version}`)
      const definition: NginxUIPlugin = { setup: () => {} }
      definitions.push(definition)
      return definition
    })
    return { injected, definitions, source }
  }

  test('does not run the script again for the same version', async () => {
    const { injected, definitions, source } = fixture()

    const first = await source.acquire({ id: 'a', version: '1' })
    const second = await source.acquire({ id: 'a', version: '1' })

    expect(injected).toEqual(['a@1'])
    expect(second).toBe(first)
    expect(definitions).toHaveLength(1)
  })

  test('runs the new script when the version changed', async () => {
    const { injected, source } = fixture()

    const first = await source.acquire({ id: 'a', version: '1' })
    const second = await source.acquire({ id: 'a', version: '2' })
    const third = await source.acquire({ id: 'a', version: '2' })

    expect(injected).toEqual(['a@1', 'a@2'])
    expect(second).not.toBe(first)
    expect(third).toBe(second)
  })

  test('forget makes the next acquire run the script', async () => {
    const { injected, source } = fixture()

    await source.acquire({ id: 'a', version: '1' })
    source.forget('a')
    await source.acquire({ id: 'a', version: '1' })

    expect(injected).toEqual(['a@1', 'a@1'])
  })

  test('remembers nothing when the bundle registered no definition', async () => {
    let calls = 0
    const source = createBundleSource<Entry>(async () => {
      calls += 1
      return undefined
    })

    expect(await source.acquire({ id: 'a', version: '1' })).toBeUndefined()
    expect(await source.acquire({ id: 'a', version: '1' })).toBeUndefined()
    expect(calls).toBe(2)
  })
})

describe('reload after unload', () => {
  test('re-runs setup without injecting a script, then injects a new version', async () => {
    setActivePinia(createPinia())
    const store = usePluginStore()

    const injected: string[] = []
    let setups = 0
    const definition: NginxUIPlugin = {
      setup(registry) {
        setups += 1
        registry.registerSlot('sidebar.footer', {})
      },
    }
    const source = createBundleSource<{ id: string, version: string }>(async entry => {
      injected.push(`${entry.id}@${entry.version}`)
      return definition
    })

    // Mirrors what the loader does around a bundle: acquire, then set up.
    const registry = {
      registerSlot: (name: string, component: object) => store.addSlot(name, 'a', component),
    } as Parameters<NginxUIPlugin['setup']>[0]
    async function load(version: string) {
      const found = await source.acquire({ id: 'a', version })
      await found?.setup(registry)
      store.setLoadState('a', 'loaded')
    }

    await load('1')
    expect(store.slots['sidebar.footer']).toHaveLength(1)

    store.removePlugin('a')
    expect(store.slots['sidebar.footer']).toBeUndefined()

    await load('1')
    expect(injected).toEqual(['a@1'])
    expect(setups).toBe(2)
    expect(store.slots['sidebar.footer']).toHaveLength(1)

    store.removePlugin('a')
    await load('2')
    expect(injected).toEqual(['a@1', 'a@2'])
  })
})

describe('chunk cache across versions', () => {
  test('keeps chunks of the same version and reloads them after a version change', async () => {
    const requested: string[] = []
    const loader = createChunkLoader({
      loadScript: async url => {
        requested.push(url)
        loader.registerChunk('a', 'search', { url })
      },
    })
    const source = (version: string) => ({ pluginId: 'a', version, chunks: { search: 'a/search.js' } })

    await loader.loadChunk(source('1'), 'search')
    await loader.loadChunk(source('1'), 'search')
    expect(requested).toEqual(['a/search.js?v=1'])

    const next = await loader.loadChunk(source('2'), 'search')
    expect(requested).toEqual(['a/search.js?v=1', 'a/search.js?v=2'])
    expect(next).toEqual({ url: 'a/search.js?v=2' })
  })
})
