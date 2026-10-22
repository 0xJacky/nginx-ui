import type { Info } from '@/api/upgrade'
import type { UpgradeCallbacks, UpgradeOptions } from '@/lib/upgrade'
import { describe, expect, mock, test } from 'bun:test'
import { matchesUpgradeTarget, runUpgrade } from '@/lib/upgrade'

Object.assign(globalThis, { $gettext: (message: string) => message })

function version(overrides: Partial<Info> = {}): Info {
  return { version: '2.9.0', short_hash: '12345678', build_id: 1, total_build: 1, instance_id: 'old-process', ...overrides }
}

function harness(overrides: Partial<UpgradeOptions> = {}) {
  let callbacks: UpgradeCallbacks
  const close = mock(() => {})
  const send = mock((_data: string) => {})
  const readVersion = mock(async () => version())
  const progress: number[] = []
  const logs: string[] = []
  const controller = new AbortController()
  const options: UpgradeOptions = {
    control: { channel: 'stable', dry_run: false },
    target: 'v2.9.1',
    signal: controller.signal,
    connect: handlers => {
      callbacks = handlers
      return close
    },
    readVersion,
    onLog: message => logs.push(message),
    onProgress: value => progress.push(value),
    verificationTimeout: 50,
    pollInterval: 1,
    ...overrides,
  }

  async function start() {
    const result = runUpgrade(options)
    // Attach a rejection handler before driving synchronous socket events.
    void result.catch(() => {})
    await Promise.resolve()
    return { result, callbacks: callbacks!, socket: { send } }
  }

  return { start, readVersion, close, send, progress, logs, controller }
}

function installationStarted(callbacks: UpgradeCallbacks, message = 'Performing core upgrade') {
  callbacks.onMessage(JSON.stringify({ status: 'info', message }))
}

describe('upgrade connection failures', () => {
  test('failed handshakes never send a command, verify a version or report 100%', async () => {
    const h = harness()
    const { result, callbacks } = await h.start()
    callbacks.onError()
    callbacks.onDisconnected()
    await expect(result).rejects.toThrow('upgrade connection failed')
    expect(h.send).not.toHaveBeenCalled()
    expect(h.readVersion).toHaveBeenCalledTimes(1)
    expect(h.progress).not.toContain(100)
    expect(h.close).toHaveBeenCalledTimes(1)
  })

  test('closing before installation is an error even after download progress', async () => {
    const h = harness()
    const { result, callbacks, socket } = await h.start()
    callbacks.onConnected(socket)
    callbacks.onMessage(JSON.stringify({ status: 'progress', progress: 100 }))
    callbacks.onDisconnected()
    await expect(result).rejects.toThrow('closed before installation started')
    expect(h.progress).not.toContain(100)
    expect(JSON.parse(h.send.mock.calls[0][0])).toEqual({ channel: 'stable', dry_run: false })
  })

  test('backend errors cannot be turned into success by a later close', async () => {
    const h = harness()
    const { result, callbacks } = await h.start()
    installationStarted(callbacks)
    callbacks.onMessage(JSON.stringify({ status: 'error', message: 'permission denied' }))
    callbacks.onDisconnected()
    await expect(result).rejects.toThrow('permission denied')
    expect(h.readVersion).toHaveBeenCalledTimes(1)
    expect(h.progress).not.toContain(100)
  })

  test.each(['not JSON', 'null', '{"status":"unexpected"}'])('rejects invalid messages: %s', async message => {
    const h = harness()
    const { result, callbacks } = await h.start()
    callbacks.onMessage(message)
    callbacks.onDisconnected()
    await expect(result).rejects.toThrow()
    expect(h.progress).not.toContain(100)
  })

  test('a stalled connection times out and is closed', async () => {
    const h = harness({ upgradeTimeout: 10 })
    const { result } = await h.start()
    await expect(result).rejects.toThrow('upgrade timed out')
    expect(h.close).toHaveBeenCalledTimes(1)
    expect(h.progress).not.toContain(100)
  })

  test('cancelling the view closes the connection', async () => {
    const h = harness()
    const { result, callbacks } = await h.start()
    h.controller.abort()
    callbacks.onDisconnected()
    await expect(result).rejects.toThrow()
    expect(h.close).toHaveBeenCalledTimes(1)
    expect(h.progress).not.toContain(100)
  })
})

describe('upgrade verification', () => {
  test.each(['Performing core upgrade', 'Docker image pull completed, upgrading...'])('verifies the new running version after %s', async message => {
    const h = harness()
    const { result, callbacks } = await h.start()
    h.readVersion.mockImplementation(async () => version({ version: '2.9.1', instance_id: 'new-process' }))
    installationStarted(callbacks, message)
    callbacks.onDisconnected()
    expect(h.progress).not.toContain(100)
    await result
    expect(h.progress.at(-1)).toBe(100)
    expect(h.close).toHaveBeenCalledTimes(1)
  })

  test('keeps polling while the old process still responds and during restart downtime', async () => {
    const h = harness()
    const { result, callbacks } = await h.start()
    h.readVersion.mockImplementationOnce(async () => version())
      .mockImplementationOnce(async () => { throw new Error('node restarting') })
      .mockImplementation(async () => version({ version: '2.9.1', instance_id: 'new-process' }))
    installationStarted(callbacks)
    callbacks.onDisconnected()
    await result
    expect(h.readVersion).toHaveBeenCalledTimes(4)
  })

  test('a live node running the wrong version never counts as success', async () => {
    const h = harness({ verificationTimeout: 10 })
    const { result, callbacks } = await h.start()
    installationStarted(callbacks)
    callbacks.onDisconnected()
    await expect(result).rejects.toThrow('Could not verify the upgrade')
    expect(h.progress).not.toContain(100)
  })

  test('reinstall waits for a different process identifier', async () => {
    const h = harness({ target: 'v2.9.0' })
    const { result, callbacks } = await h.start()
    h.readVersion.mockImplementationOnce(async () => version())
      .mockImplementation(async () => version({ instance_id: 'new-process' }))
    installationStarted(callbacks)
    callbacks.onDisconnected()
    await result
    expect(h.readVersion).toHaveBeenCalledTimes(3)
  })

  test('an unchanged reinstall is unverified even when its version matches', async () => {
    const h = harness({ target: 'v2.9.0', verificationTimeout: 10 })
    const { result, callbacks } = await h.start()
    installationStarted(callbacks)
    callbacks.onDisconnected()
    await expect(result).rejects.toThrow('Could not verify the upgrade')
    expect(h.progress).not.toContain(100)
  })

  test('supports an older node without a process identifier when its build changes', async () => {
    let reads = 0
    const legacy = harness({
      readVersion: async () => version({ instance_id: undefined, version: ++reads === 1 ? '2.9.0' : '2.9.1' }),
    })
    const started = await legacy.start()
    installationStarted(started.callbacks)
    started.callbacks.onDisconnected()
    await started.result
    expect(legacy.progress.at(-1)).toBe(100)
  })

  test('dry run completes without a restart or version verification', async () => {
    const h = harness({ control: { channel: 'stable', dry_run: true } })
    const { result, callbacks } = await h.start()
    installationStarted(callbacks)
    callbacks.onDisconnected()
    await result
    expect(h.readVersion).toHaveBeenCalledTimes(1)
  })

  test('cancelling during verification stops polling', async () => {
    const h = harness({ pollInterval: 50 })
    const { result, callbacks } = await h.start()
    installationStarted(callbacks)
    callbacks.onDisconnected()
    h.controller.abort()
    await expect(result).rejects.toThrow()
    expect(h.readVersion).toHaveBeenCalledTimes(1)
    expect(h.progress).not.toContain(100)
  })
})

describe('upgrade target matching', () => {
  test('compares release versions exactly while accepting a v prefix', () => {
    expect(matchesUpgradeTarget(version(), 'v2.9.0')).toBe(true)
    expect(matchesUpgradeTarget(version(), '2.9.0')).toBe(true)
    expect(matchesUpgradeTarget(version(), 'v2.9.1')).toBe(false)
    expect(matchesUpgradeTarget(version(), '')).toBe(false)
  })

  test('compares development releases by commit hash', () => {
    expect(matchesUpgradeTarget(version({ short_hash: 'ABCDEF12' }), 'sha-abcdef1')).toBe(true)
    expect(matchesUpgradeTarget(version(), 'sha-abcdef1')).toBe(false)
    expect(matchesUpgradeTarget(version(), 'sha-')).toBe(false)
  })
})
