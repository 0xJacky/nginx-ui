/**
 * On-demand chunks of a plugin bundle (spec WEB-13).
 *
 * A chunk is one more IIFE file. It hands its exports over by calling
 * `window.NginxUI.registerChunk(pluginId, name, exports)` while its script
 * executes, and `loadChunk` reads them right after the script finished.
 */

export type ChunkExports = Record<string, unknown>

/** What the host knows about the plugin asking for a chunk. */
export interface ChunkSource {
  pluginId: string
  /** Plugin version, applied to the chunk address like it is for the entry. */
  version?: string
  /** Chunk name to address, only names the plugin declared. */
  chunks?: Record<string, string>
}

/** Runs one task at a time, in the order they were queued. */
export interface ScriptQueue {
  run: <T>(task: () => Promise<T>) => Promise<T>
}

export function createScriptQueue(): ScriptQueue {
  let tail: Promise<unknown> = Promise.resolve()

  return {
    run<T>(task: () => Promise<T>): Promise<T> {
      const result = tail.then(task, task)
      // A failing task must not block the ones queued behind it.
      tail = result.catch(() => {})
      return result
    },
  }
}

export function withVersion(url: string, version?: string) {
  if (!version)
    return url

  return `${url}${url.includes('?') ? '&' : '?'}v=${encodeURIComponent(version)}`
}

/** Loads a classic script and resolves once it executed. */
export function injectScript(url: string): Promise<void> {
  return new Promise((resolve, reject) => {
    const script = document.createElement('script')
    script.src = url
    script.async = false
    script.onload = () => resolve()
    script.onerror = () => reject(new Error(`Failed to load plugin script ${url}`))
    document.head.appendChild(script)
  })
}

export interface ChunkLoaderOptions {
  /** Executes the script at `url`. */
  loadScript: (url: string) => Promise<void>
  /** Shared with the entry bundle loads, so two scripts never run at once. */
  queue?: ScriptQueue
}

export interface ChunkLoader {
  registerChunk: (pluginId: string, name: string, exports: ChunkExports) => void
  loadChunk: (source: ChunkSource, name: string) => Promise<ChunkExports>
}

export function createChunkLoader(options: ChunkLoaderOptions): ChunkLoader {
  const queue = options.queue ?? createScriptQueue()
  /** Exports handed over by the script that is running. */
  const handedOver = new Map<string, ChunkExports>()
  /** One promise per chunk, so every caller gets the same exports. */
  const loads = new Map<string, Promise<ChunkExports>>()
  /** Version the cached chunks of a plugin belong to. */
  const loadedVersions = new Map<string, string>()

  const keyOf = (pluginId: string, name: string) => `${pluginId}\u0000${name}`

  function registerChunk(pluginId: string, name: string, exports: ChunkExports) {
    if (typeof pluginId !== 'string' || typeof name !== 'string' || !exports || typeof exports !== 'object') {
      console.warn('[plugin] registerChunk ignored: a plugin id, a chunk name and an exports object are required')
      return
    }

    handedOver.set(keyOf(pluginId, name), exports)
  }

  function run(source: ChunkSource, name: string, url: string): Promise<ChunkExports> {
    return queue.run(async () => {
      handedOver.clear()
      try {
        await options.loadScript(withVersion(url, source.version))
        const exports = handedOver.get(keyOf(source.pluginId, name))
        if (!exports)
          throw new Error(`Chunk ${name} of ${source.pluginId} did not call registerChunk with its own id and name`)

        return exports
      }
      finally {
        handedOver.clear()
      }
    })
  }

  function loadChunk(source: ChunkSource, name: string): Promise<ChunkExports> {
    const chunks = source.chunks ?? {}
    const url = typeof name === 'string' && Object.hasOwn(chunks, name) ? chunks[name] : ''
    if (!url)
      return Promise.reject(new Error(`Chunk ${String(name)} is not declared by ${source.pluginId}`))

    // Chunks of another version are not the ones the new bundle expects.
    const version = source.version ?? ''
    if (loadedVersions.get(source.pluginId) !== version) {
      for (const cached of [...loads.keys()]) {
        if (cached.startsWith(`${source.pluginId}\u0000`))
          loads.delete(cached)
      }
      loadedVersions.set(source.pluginId, version)
    }

    const key = keyOf(source.pluginId, name)
    const existing = loads.get(key)
    if (existing)
      return existing

    const load = run(source, name, url)
    loads.set(key, load)
    // A failed chunk may be asked for again.
    load.catch(() => {
      if (loads.get(key) === load)
        loads.delete(key)
    })

    return load
  }

  return { registerChunk, loadChunk }
}

/** Queue every script of the page runs through: entry bundles and chunks. */
export const scriptQueue = createScriptQueue()

export const chunkLoader = createChunkLoader({ loadScript: injectScript, queue: scriptQueue })
