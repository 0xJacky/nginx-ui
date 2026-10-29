import { afterEach, beforeEach, describe, expect, mock, test } from 'bun:test'
import { createPinia, defineStore, setActivePinia } from 'pinia'
import { effectScope, onScopeDispose, ref } from 'vue'

const sockets: Socket[] = []

class Socket {
  static OPEN = 1
  static CLOSED = 3
  readyState = 0
  onopen?: () => void
  onclose?: (event: { code: number }) => void
  onmessage?: (event: { data: string }) => void
  onerror?: (event: Event) => void
  send = mock((_data: string) => {})
  close = mock(() => {
    this.readyState = Socket.CLOSED
    this.onclose?.({ code: 1000 })
  })

  constructor(public url: string) {
    sockets.push(this)
  }
}

const location = { protocol: 'https:', host: 'controller.test', href: 'https://controller.test/panel/#/nodes' }
Object.assign(globalThis, {
  window: Object.assign(new EventTarget(), { location, WebSocket: Socket }),
  document: new EventTarget(),
  location,
  WebSocket: Socket,
  onScopeDispose,
  $gettext: (message: string) => message,
})

const useSettingsStore = defineStore('upgrade-settings', () => ({
  node: ref({ id: 99 }),
}))
const useUserStore = defineStore('upgrade-user', () => ({
  token: ref('browser-token'),
  shortToken: ref(''),
}))
mock.module('../src/pinia', () => ({ useSettingsStore, useUserStore }))
const authorize = mock(async () => 'secure-session/+?&=')
mock.module('../src/components/TwoFA/use2FAModal', () => ({
  default: () => ({ open: authorize }),
}))

interface RequestConfig {
  headers?: Record<string, number>
  skipNodeProxy: boolean
  signal: AbortSignal
  timeout: number
}

function version(instance = 'before') {
  return {
    version: '2.9.0',
    short_hash: '12345678',
    build_id: 1,
    total_build: 1,
    instance_id: instance,
  }
}

const get = mock(async (_url: string, _config: RequestConfig) => version())
mock.module('@uozi-admin/request', () => ({
  http: { get },
}))

// Exercise the real upgrade API, shared URL builder, VueUse connection and
// composable. Only the browser transport, stores and 2FA UI are replaced.
const { useUpgrade } = await import('../src/composables/useUpgrade')
const scopes: ReturnType<typeof effectScope>[] = []

beforeEach(() => {
  setActivePinia(createPinia())
  sockets.length = 0
  get.mockClear()
  get.mockImplementation(async () => version())
  authorize.mockClear()
  authorize.mockImplementation(async () => 'secure-session/+?&=')
})

afterEach(() => {
  scopes.splice(0).forEach(scope => scope.stop())
})

function setup() {
  const scope = effectScope()
  scopes.push(scope)
  const client = scope.run(useUpgrade)!
  const progress: number[] = []
  const options = {
    nodeId: 7,
    secureSessionId: 'secure-session/+?&=',
    target: 'v2.9.0',
    control: { channel: 'stable', dry_run: true },
    onLog: mock((_message: string) => {}),
    onProgress: (value: number) => progress.push(value),
  }
  return { client, options, scope, progress }
}

async function start(h: ReturnType<typeof setup>) {
  const result = h.client.perform(h.options)
  void result.catch(() => {})
  await Promise.resolve()
  const socket = sockets.at(-1)!
  expect(socket).toBeDefined()
  socket.readyState = Socket.OPEN
  socket.onopen?.()
  return {
    result,
    socket,
    url: new URL(socket.url),
  }
}

function finishDryRun(socket: Socket) {
  socket.onmessage?.({
    data: JSON.stringify({ status: 'info', message: 'Performing core upgrade' }),
  })
  socket.close()
}

describe('upgrade client integration', () => {
  test('carries the verified session in the handshake and sends the control once', async () => {
    const h = setup()
    h.options.secureSessionId = await h.client.authorize()
    const { result, socket, url } = await start(h)
    expect(authorize).toHaveBeenCalledTimes(1)
    expect(url.searchParams.get('X-Secure-Session-ID')).toBe('secure-session/+?&=')
    expect(url.searchParams.get('token')).toBe(btoa('browser-token').replace(/=+$/, ''))
    expect(socket.send).toHaveBeenCalledTimes(1)
    expect(JSON.parse(socket.send.mock.calls[0][0])).toEqual({ channel: 'stable', dry_run: true })
    finishDryRun(socket)
    await result
    expect(h.progress.at(-1)).toBe(100)
  })

  test('pins both HTTP version checks and the socket to the requested node', async () => {
    const h = setup()
    const { result, socket, url } = await start(h)
    expect(useSettingsStore().node.id).toBe(99)
    expect(url.searchParams.get('x_node_id')).toBe('7')
    expect(get.mock.calls[0][0]).toBe('/upgrade/current')
    expect(get.mock.calls[0][1]).toMatchObject({
      skipNodeProxy: true,
      headers: { 'X-Node-ID': 7 },
      timeout: 5000,
    })
    finishDryRun(socket)
    await result
  })

  test('explicit local upgrades override an active remote node', async () => {
    const h = setup()
    h.options.nodeId = 0
    const { result, socket, url } = await start(h)
    expect(url.searchParams.get('x_node_id')).toBe('0')
    expect(get.mock.calls[0][1].skipNodeProxy).toBe(true)
    expect(get.mock.calls[0][1].headers).toBeUndefined()
    finishDryRun(socket)
    await result
  })

  test('a user without 2FA can upgrade without an empty session query parameter', async () => {
    const h = setup()
    authorize.mockImplementation(async () => '')
    h.options.secureSessionId = await h.client.authorize()
    const { result, socket, url } = await start(h)
    expect(url.searchParams.has('X-Secure-Session-ID')).toBe(false)
    finishDryRun(socket)
    await result
  })

  test('the real VueUse error and close callbacks reject instead of reporting success', async () => {
    const h = setup()
    const { result, socket } = await start(h)
    socket.onerror?.(new Event('error'))
    socket.onclose?.({ code: 1006 })
    await expect(result).rejects.toThrow('upgrade connection failed')
    expect(h.progress).not.toContain(100)
    expect(get).toHaveBeenCalledTimes(1)
  })

  test('disposing the page aborts HTTP requests and closes the socket', async () => {
    const h = setup()
    const { result, socket } = await start(h)
    h.scope.stop()
    await expect(result).rejects.toThrow()
    expect(get.mock.calls[0][1].signal.aborted).toBe(true)
    expect(socket.close).toHaveBeenCalledTimes(1)
    expect(h.progress).not.toContain(100)
  })

  test('selection, credentials and controls are snapshotted before awaiting the version request', async () => {
    let release!: (info: ReturnType<typeof version>) => void
    get.mockImplementation(() => new Promise(resolve => {
      release = resolve
    }))
    const h = setup()
    const result = h.client.perform(h.options)
    void result.catch(() => {})
    useSettingsStore().node.id = 42
    h.options.nodeId = 42
    h.options.secureSessionId = 'replacement-session'
    h.options.control.channel = 'dev'
    release(version())
    await Promise.resolve()
    const socket = sockets.at(-1)!
    const url = new URL(socket.url)
    socket.readyState = Socket.OPEN
    socket.onopen?.()
    expect(url.searchParams.get('x_node_id')).toBe('7')
    expect(url.searchParams.get('X-Secure-Session-ID')).toBe('secure-session/+?&=')
    expect(JSON.parse(socket.send.mock.calls[0][0]).channel).toBe('stable')
    finishDryRun(socket)
    await result
  })

  test('a failed baseline version request never starts an upgrade socket', async () => {
    const h = setup()
    get.mockImplementation(async () => {
      throw new Error('node offline')
    })
    await expect(h.client.perform(h.options)).rejects.toThrow('node offline')
    expect(sockets).toHaveLength(0)
    expect(h.progress).not.toContain(100)
  })

  test('handshake errors before open never send an upgrade command or report success', async () => {
    const h = setup()
    const result = h.client.perform(h.options)
    void result.catch(() => {})
    await Promise.resolve()
    const socket = sockets.at(-1)!
    socket.onerror?.(new Event('error'))
    socket.onclose?.({ code: 1006 })
    await expect(result).rejects.toThrow('upgrade connection failed')
    expect(socket.send).not.toHaveBeenCalled()
    expect(h.progress).not.toContain(100)
    expect(sockets).toHaveLength(1)
  })

  test('post-restart version requests keep using the original target after a node switch', async () => {
    const h = setup()
    h.options.target = 'v2.9.1'
    h.options.control.dry_run = false
    const { result, socket } = await start(h)
    useSettingsStore().node.id = 42
    get.mockImplementation(async () => ({
      ...version('after'),
      version: '2.9.1',
    }))
    finishDryRun(socket)
    await result
    expect(get).toHaveBeenCalledTimes(2)
    for (const [url, config] of get.mock.calls) {
      expect(url).toBe('/upgrade/current')
      expect(config).toMatchObject({
        headers: { 'X-Node-ID': 7 },
        skipNodeProxy: true,
      })
    }
    expect(h.progress.at(-1)).toBe(100)
    expect(sockets).toHaveLength(1)
  })
})
