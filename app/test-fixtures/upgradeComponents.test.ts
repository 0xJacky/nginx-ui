import type { Ref } from 'vue'
import { Buffer } from 'node:buffer'
import { readFileSync } from 'node:fs'
import { compileScript, parse } from '@vue/compiler-sfc'
import { Transpiler } from 'bun'
import { beforeEach, describe, expect, mock, test } from 'bun:test'
import { computed, effectScope, nextTick, ref, watch } from 'vue'

interface UpgradeRequest {
  nodeId: number
  secureSessionId: string
  target: string
  onProgress: (progress: number) => void
}

class TwoFACancelledError extends Error {}
const authorize = mock(async () => 'session-1')
const perform = mock(async (_options: UpgradeRequest) => {})
const saveChannel = mock(async (_channel: string) => ({}))
const reload = mock(() => {})
mock.module('../src/composables/useUpgrade', () => ({
  useUpgrade: () => ({ authorize, perform }),
}))
mock.module('../src/components/TwoFA/use2FAModal', () => ({ TwoFACancelledError }))
mock.module('../src/api/upgrade', () => ({
  default: {
    get_latest_release: async () => ({ name: 'v2.9.1', tag_name: 'v2.9.1', body: '' }),
    get_channel: async () => ({ channel: 'stable' }),
    save_channel: saveChannel,
  },
}))

let logs: string[] = []
let query: Record<string, string> = {}
Object.assign(globalThis, {
  ref,
  computed,
  watch,
  nextTick,
  useRoute: () => ({ query }),
  location: { reload },
  useTemplateRef: () => ref({
    innerHTML: '',
    appendChild: (paragraph: { text: string }) => logs.push(paragraph.text),
    scroll() {},
  }),
  document: {
    createElement: () => ({
      text: '',
      appendChild(text: string) { this.text += text },
    }),
    createTextNode: (text: string) => text,
  },
  $gettext: (message: string, args?: Record<string, string>) => message.replace(/%\{(\w+)\}/g, (_, key: string) => args?.[key] ?? ''),
})

// Compile and execute the actual component's setup function. This tests its
// control flow without duplicating it or requiring a browser DOM implementation.
async function loadComponent(path: string) {
  const source = readFileSync(new URL(path, import.meta.url), 'utf8')
  const { descriptor } = parse(source)
  const compiled = compileScript(descriptor, { id: path })
  const code = new Transpiler({ loader: 'ts' }).transformSync(compiled.content)
  const module = await import(`data:text/javascript;base64,${Buffer.from(code).toString('base64')}`)
  return module.default
}

const component = await loadComponent('../src/views/node/BatchUpgrader.vue')
const systemComponent = await loadComponent('../src/views/system/Upgrade.vue')

interface BatchState {
  performUpgrade: () => Promise<void>
  progressStatus: Ref<string>
  upgrading: Ref<boolean>
}

interface BatchExposed {
  open: (ids: Ref<number[]>, rows: Ref<{ id: number, name: string }[]>) => void
}

beforeEach(() => {
  logs = []
  query = {}
  reload.mockClear()
  saveChannel.mockClear()
  authorize.mockClear()
  authorize.mockImplementation(async () => 'session-1')
  perform.mockClear()
  perform.mockImplementation(async options => {
    options.onProgress(100)
  })
})

async function setup(ids = [11, 22], rows = [
  { id: 11, name: 'Node A' },
  { id: 22, name: 'Node B' },
]) {
  const emit = mock((_event: string) => {})
  let exposed!: BatchExposed
  const scope = effectScope()
  const state = scope.run(() => component.setup({}, {
    expose: (value: BatchExposed) => {
      exposed = value
    },
    emit,
  })) as BatchState
  exposed.open(ref(ids), ref(rows))
  await nextTick()
  return { state, emit, scope, exposed }
}

describe('batch upgrade component', () => {
  test('a failed node leaves the batch failed even when the following node succeeds', async () => {
    const h = await setup()
    perform.mockImplementationOnce(async () => {
      throw new Error('handshake rejected')
    })
    await h.state.performUpgrade()
    expect(perform.mock.calls.map(([request]) => request.nodeId)).toEqual([11, 22])
    expect(h.state.progressStatus.value).toBe('exception')
    expect(h.emit).not.toHaveBeenCalled()
    expect(logs.some(message => message.includes('Node A') && message.includes('handshake rejected'))).toBe(true)
    expect(logs.some(message => message.includes('Node A successfully'))).toBe(false)
    expect(logs.some(message => message.includes('Node B successfully'))).toBe(true)
    expect(h.state.upgrading.value).toBe(false)
    h.scope.stop()
  })

  test('all failed nodes never emit success or print a successful node', async () => {
    const h = await setup()
    perform.mockImplementation(async () => {
      throw new Error('node offline')
    })
    await h.state.performUpgrade()
    expect(h.state.progressStatus.value).toBe('exception')
    expect(h.emit).not.toHaveBeenCalled()
    expect(logs.some(message => message.includes('successfully'))).toBe(false)
    h.scope.stop()
  })

  test('all verified nodes emit success once', async () => {
    const h = await setup()
    await h.state.performUpgrade()
    expect(h.state.progressStatus.value).toBe('success')
    expect(h.emit).toHaveBeenCalledTimes(1)
    expect(h.emit).toHaveBeenCalledWith('success')
    h.scope.stop()
  })

  test('cancelled authorization sends no commands and unlocks the dialog', async () => {
    const h = await setup()
    authorize.mockImplementation(async () => {
      throw new TwoFACancelledError('cancelled')
    })
    await h.state.performUpgrade()
    expect(perform).not.toHaveBeenCalled()
    expect(h.emit).not.toHaveBeenCalled()
    expect(h.state.upgrading.value).toBe(false)
    h.scope.stop()
  })

  test('the logged node and requested node stay paired when selection arrays have different orders', async () => {
    const h = await setup([22, 11])
    perform.mockImplementation(async options => {
      if (options.nodeId === 11)
        throw new Error('Node A offline')
      options.onProgress(100)
    })
    await h.state.performUpgrade()
    expect(logs.some(message => message.includes('Node A') && message.includes('Node A offline'))).toBe(true)
    expect(logs.some(message => message.includes('Node B successfully'))).toBe(true)
    expect(logs.some(message => message.includes('Node A successfully'))).toBe(false)
    h.scope.stop()
  })

  test('each subsequent node revalidates the secure session after a potentially long upgrade', async () => {
    const h = await setup()
    authorize.mockImplementationOnce(async () => 'session-1').mockImplementation(async () => 'session-2')
    await h.state.performUpgrade()
    expect(perform.mock.calls.map(([request]) => request.secureSessionId)).toEqual(['session-1', 'session-2'])
    h.scope.stop()
  })

  test('empty selections never start an upgrade or report success', async () => {
    const h = await setup([], [])
    await h.state.performUpgrade()
    expect(authorize).not.toHaveBeenCalled()
    expect(perform).not.toHaveBeenCalled()
    expect(h.emit).not.toHaveBeenCalled()
    expect(h.state.progressStatus.value).not.toBe('success')
    h.scope.stop()
  })

  test('reopening the dialog during an upgrade cannot replace the active node list', async () => {
    const h = await setup()
    perform.mockImplementationOnce(async () => {
      h.exposed.open(ref([33]), ref([
        { id: 33, name: 'Other node' },
      ]))
    })
    await h.state.performUpgrade()
    expect(perform.mock.calls.map(([request]) => request.nodeId)).toEqual([11, 22])
    h.scope.stop()
  })

  test('cancelling reauthorization stops the remaining nodes and leaves no success event', async () => {
    const h = await setup()
    authorize.mockImplementationOnce(async () => 'session-1').mockImplementation(async () => {
      throw new TwoFACancelledError('cancelled')
    })
    await h.state.performUpgrade()
    expect(perform).toHaveBeenCalledTimes(1)
    expect(h.state.progressStatus.value).toBe('exception')
    expect(h.emit).not.toHaveBeenCalled()
    expect(h.state.upgrading.value).toBe(false)
    h.scope.stop()
  })

  test('duplicate clicks cannot start a second batch', async () => {
    const h = await setup()
    const first = h.state.performUpgrade()
    await h.state.performUpgrade()
    await first
    expect(perform).toHaveBeenCalledTimes(2)
    expect(h.emit).toHaveBeenCalledTimes(1)
    h.scope.stop()
  })
})

async function setupSystem() {
  const scope = effectScope()
  const state = scope.run(() => systemComponent.setup({}, {
    expose() {},
  })) as BatchState
  await nextTick()
  await nextTick()
  return { scope, state }
}

describe('single-node upgrade component', () => {
  test('failed upgrades never reload or report success', async () => {
    const h = await setupSystem()
    perform.mockImplementation(async () => {
      throw new Error('failed to verify version')
    })
    await h.state.performUpgrade()
    expect(h.state.progressStatus.value).toBe('exception')
    expect(logs).toContain('failed to verify version')
    expect(logs.some(message => message.includes('successfully'))).toBe(false)
    expect(reload).not.toHaveBeenCalled()
    h.scope.stop()
  })

  test('successful verification reloads once and uses the authenticated session and target', async () => {
    const h = await setupSystem()
    await h.state.performUpgrade()
    expect(perform.mock.calls[0][0]).toMatchObject({ secureSessionId: 'session-1', target: 'v2.9.1' })
    expect(h.state.progressStatus.value).toBe('success')
    expect(reload).toHaveBeenCalledTimes(1)
    h.scope.stop()
  })

  test('dry runs do not reload or claim that a version was installed', async () => {
    query.dry_run = '1'
    const h = await setupSystem()
    await h.state.performUpgrade()
    expect(logs).toContain('Dry run completed. No changes were made.')
    expect(logs).not.toContain('Upgraded successfully')
    expect(reload).not.toHaveBeenCalled()
    h.scope.stop()
  })

  test('cancelled authorization sends no commands or settings changes', async () => {
    const h = await setupSystem()
    authorize.mockImplementation(async () => {
      throw new TwoFACancelledError('cancelled')
    })
    await h.state.performUpgrade()
    expect(perform).not.toHaveBeenCalled()
    expect(saveChannel).not.toHaveBeenCalled()
    expect(reload).not.toHaveBeenCalled()
    expect(h.state.upgrading.value).toBe(false)
    h.scope.stop()
  })

  test('duplicate clicks cannot start multiple single-node upgrades', async () => {
    const h = await setupSystem()
    const first = h.state.performUpgrade()
    await h.state.performUpgrade()
    await first
    expect(perform).toHaveBeenCalledTimes(1)
    expect(reload).toHaveBeenCalledTimes(1)
    h.scope.stop()
  })
})
