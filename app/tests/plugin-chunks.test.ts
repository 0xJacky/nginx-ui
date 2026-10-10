import { describe, expect, test } from 'bun:test'
import { createChunkLoader, createScriptQueue, withVersion } from '../src/plugin/chunks'

const source = {
  pluginId: 'com.example.logs',
  version: '1.2.0',
  chunks: {
    search: 'plugins/com.example.logs/webapp/search.js',
    dashboard: 'plugins/com.example.logs/webapp/dashboard.js',
  },
}

type Loader = ReturnType<typeof createChunkLoader>

/** A script loader whose scripts call registerChunk like a real chunk does. */
function fakeScripts(register: (url: string) => void) {
  const requested: string[] = []
  const loadScript = async (url: string) => {
    requested.push(url)
    await Promise.resolve()
    register(url)
  }
  return { requested, loadScript }
}

describe('withVersion', () => {
  test('appends the version like the entry bundle does', () => {
    expect(withVersion('a.js', '1.0.0')).toBe('a.js?v=1.0.0')
    expect(withVersion('a.js?x=1', '1 0')).toBe('a.js?x=1&v=1%200')
    expect(withVersion('a.js', '')).toBe('a.js')
    expect(withVersion('a.js', undefined)).toBe('a.js')
  })
})

describe('loadChunk', () => {
  test('resolves with the exports the chunk registered and applies the version', async () => {
    const exportsOfSearch = { Search: () => 'search' }
    let loader: Loader
    const { requested, loadScript } = fakeScripts(() => {
      loader.registerChunk('com.example.logs', 'search', exportsOfSearch)
    })
    loader = createChunkLoader({ loadScript })

    const result = await loader.loadChunk(source, 'search')

    expect(result).toBe(exportsOfSearch)
    expect(requested).toEqual(['plugins/com.example.logs/webapp/search.js?v=1.2.0'])
  })

  test('rejects an undeclared name without any request', async () => {
    const { requested, loadScript } = fakeScripts(() => {})
    const loader = createChunkLoader({ loadScript })

    await expect(loader.loadChunk(source, 'missing')).rejects.toThrow('not declared')
    await expect(loader.loadChunk(source, 'toString')).rejects.toThrow('not declared')
    await expect(loader.loadChunk({ pluginId: 'x' }, 'search')).rejects.toThrow('not declared')
    expect(requested).toEqual([])
  })

  test('loads a chunk once and shares the promise between callers', async () => {
    const shared = { value: 1 }
    let loader: Loader
    const { requested, loadScript } = fakeScripts(() => loader.registerChunk('com.example.logs', 'search', shared))
    loader = createChunkLoader({ loadScript })

    const [first, second] = await Promise.all([loader.loadChunk(source, 'search'), loader.loadChunk(source, 'search')])
    const later = await loader.loadChunk(source, 'search')

    expect(first).toBe(shared)
    expect(second).toBe(shared)
    expect(later).toBe(shared)
    expect(requested).toHaveLength(1)
  })

  test('keeps chunks of different plugins and names apart', async () => {
    let loader: Loader
    const { requested, loadScript } = fakeScripts(url => {
      const name = url.includes('search') ? 'search' : 'dashboard'
      const plugin = url.includes('other') ? 'com.example.other' : 'com.example.logs'
      loader.registerChunk(plugin, name, { url })
    })
    loader = createChunkLoader({ loadScript })
    const other = { pluginId: 'com.example.other', chunks: { search: 'other/search.js' } }

    const [a, b, c] = await Promise.all([
      loader.loadChunk(source, 'search'),
      loader.loadChunk(source, 'dashboard'),
      loader.loadChunk(other, 'search'),
    ])

    expect(new Set([a, b, c]).size).toBe(3)
    expect(requested).toHaveLength(3)
  })

  test('rejects when the script does not register with its own id and name, then allows a retry', async () => {
    let mode: 'none' | 'wrong-id' | 'wrong-name' | 'ok' = 'none'
    let loader: Loader
    const { requested, loadScript } = fakeScripts(() => {
      if (mode === 'wrong-id')
        loader.registerChunk('com.example.evil', 'search', {})
      else if (mode === 'wrong-name')
        loader.registerChunk('com.example.logs', 'dashboard', {})
      else if (mode === 'ok')
        loader.registerChunk('com.example.logs', 'search', { ok: true })
    })
    loader = createChunkLoader({ loadScript })

    for (mode of ['none', 'wrong-id', 'wrong-name'] as const)
      await expect(loader.loadChunk(source, 'search')).rejects.toThrow('did not call registerChunk')

    mode = 'ok'
    expect(await loader.loadChunk(source, 'search')).toEqual({ ok: true })
    expect(requested).toHaveLength(4)
  })

  test('a leftover registration never satisfies a later chunk', async () => {
    let loader: Loader
    let calls = 0
    const { loadScript } = fakeScripts(() => {
      calls += 1
      // The first script registers the wrong chunk, the second registers nothing.
      if (calls === 1)
        loader.registerChunk('com.example.logs', 'dashboard', { stale: true })
    })
    loader = createChunkLoader({ loadScript })

    await expect(loader.loadChunk(source, 'search')).rejects.toThrow()
    await expect(loader.loadChunk(source, 'dashboard')).rejects.toThrow()
  })

  test('rejects and retries when the file fails to load', async () => {
    let fail = true
    const requested: string[] = []
    const loader: Loader = createChunkLoader({
      loadScript: async url => {
        requested.push(url)
        if (fail)
          throw new Error('404')
        loader.registerChunk('com.example.logs', 'search', { ok: true })
      },
    })

    await expect(loader.loadChunk(source, 'search')).rejects.toThrow('404')
    fail = false
    expect(await loader.loadChunk(source, 'search')).toEqual({ ok: true })
    expect(requested).toHaveLength(2)
  })

  test('a failing chunk does not affect the others', async () => {
    const loader: Loader = createChunkLoader({
      loadScript: async url => {
        if (url.includes('search'))
          throw new Error('boom')
        loader.registerChunk('com.example.logs', 'dashboard', { ok: true })
      },
    })

    const [search, dashboard] = await Promise.allSettled([
      loader.loadChunk(source, 'search'),
      loader.loadChunk(source, 'dashboard'),
    ])

    expect(search.status).toBe('rejected')
    expect(dashboard.status).toBe('fulfilled')
  })

  test('ignores a registerChunk call with bad arguments', async () => {
    const loader = createChunkLoader({ loadScript: async () => {} })
    const warn = console.warn
    console.warn = () => {}
    try {
      loader.registerChunk('a', 'b', null as never)
      loader.registerChunk(1 as never, 'b', {})
    }
    finally {
      console.warn = warn
    }
    await expect(loader.loadChunk(source, 'search')).rejects.toThrow()
  })

  test('never runs two scripts at the same time, also across the shared queue', async () => {
    const queue = createScriptQueue()
    let running = 0
    let maxRunning = 0
    const loader: Loader = createChunkLoader({
      queue,
      loadScript: async url => {
        running += 1
        maxRunning = Math.max(maxRunning, running)
        await new Promise(resolve => setTimeout(resolve, 5))
        running -= 1
        loader.registerChunk('com.example.logs', url.includes('search') ? 'search' : 'dashboard', {})
      },
    })

    // An entry bundle load queued on the same queue.
    const entry = queue.run(async () => {
      running += 1
      maxRunning = Math.max(maxRunning, running)
      await new Promise(resolve => setTimeout(resolve, 5))
      running -= 1
    })

    await Promise.all([entry, loader.loadChunk(source, 'search'), loader.loadChunk(source, 'dashboard')])

    expect(maxRunning).toBe(1)
  })
})

describe('createScriptQueue', () => {
  test('runs tasks in order and keeps going after a failure', async () => {
    const queue = createScriptQueue()
    const order: number[] = []

    const first = queue.run(async () => {
      await new Promise(resolve => setTimeout(resolve, 10))
      order.push(1)
      throw new Error('first failed')
    })
    const second = queue.run(async () => {
      order.push(2)
      return 'done'
    })

    await expect(first).rejects.toThrow('first failed')
    expect(await second).toBe('done')
    expect(order).toEqual([1, 2])
  })
})
