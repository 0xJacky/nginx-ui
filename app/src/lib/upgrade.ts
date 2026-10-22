import type { Info } from '@/api/upgrade'

export interface UpgradeControl {
  channel: string
  dry_run: boolean
  test_commit_and_restart?: boolean
}

export interface UpgradeCallbacks {
  onConnected: (socket: Pick<WebSocket, 'send'>) => void
  onMessage: (data: string) => void
  onDisconnected: () => void
  onError: () => void
}

export interface UpgradeOptions {
  control: UpgradeControl
  target: string
  signal: AbortSignal
  connect: (callbacks: UpgradeCallbacks) => () => void
  readVersion: (signal: AbortSignal) => Promise<Info>
  onLog: (message: string) => void
  onProgress: (progress: number) => void
  upgradeTimeout?: number
  verificationTimeout?: number
  pollInterval?: number
}

function isVersionInfo(info: unknown): info is Info {
  return !!info && typeof info === 'object'
    && 'version' in info && typeof info.version === 'string' && !!info.version
    && 'short_hash' in info && typeof info.short_hash === 'string'
    && (!('instance_id' in info) || info.instance_id === undefined || typeof info.instance_id === 'string')
}

function isUpgradeTarget(target: string): boolean {
  return /^sha-[\da-f]{7,40}$/i.test(target)
    || /^v?\d+\.\d+\.\d+(?:-[\da-z.-]+)?(?:\+[\da-z.-]+)?$/i.test(target)
}

export function matchesUpgradeTarget(info: Info, target: string): boolean {
  if (!isVersionInfo(info) || !isUpgradeTarget(target))
    return false
  if (target.toLowerCase().startsWith('sha-')) {
    const hash = target.slice(4).toLowerCase()
    return hash.length >= 7 && info.short_hash.toLowerCase().startsWith(hash)
  }

  return !!target && info.version.replace(/^v/, '') === target.replace(/^v/, '')
}

function hasRestarted(before: Info, after: Info): boolean {
  if (after.instance_id)
    return after.instance_id !== before.instance_id

  // Older nodes have no process identifier. A changed running build still
  // proves that the upgrade took effect, but an unchanged reinstall does not.
  return before.version !== after.version || before.short_hash !== after.short_hash
}

function delay(ms: number, signal: AbortSignal): Promise<void> {
  signal.throwIfAborted()
  return new Promise((resolve, reject) => {
    let timer: ReturnType<typeof setTimeout>
    const onAbort = () => {
      clearTimeout(timer)
      reject(signal.reason)
    }
    timer = setTimeout(() => {
      signal.removeEventListener('abort', onAbort)
      resolve()
    }, ms)
    signal.addEventListener('abort', onAbort, { once: true })
  })
}

async function verifyUpgrade(options: UpgradeOptions, before: Info): Promise<void> {
  options.onLog($gettext('Waiting for the node to restart and report the target version...'))
  options.onProgress(95)
  while (true) {
    await delay(options.pollInterval ?? 1000, options.signal)
    try {
      const current = await options.readVersion(options.signal)
      options.signal.throwIfAborted()
      if (matchesUpgradeTarget(current, options.target) && hasRestarted(before, current))
        return
    }
    catch {
      options.signal.throwIfAborted()
    }
  }
}

export async function runUpgrade(options: UpgradeOptions): Promise<void> {
  options.signal.throwIfAborted()
  const control = { ...options.control }
  const target = options.target
  if (!isUpgradeTarget(target))
    throw new Error($gettext('Invalid upgrade target. Check for a release before upgrading.'))

  return new Promise((resolve, reject) => {
    const controller = new AbortController()
    const signal = controller.signal
    let settled = false
    let verifying = false
    let readyToRestart = false
    let progress = 0
    let close: (() => void) | undefined
    let verificationTimeout: ReturnType<typeof setTimeout> | undefined

    const timeout = setTimeout(() => {
      finish(new Error($gettext('The upgrade timed out. Check the node logs.')))
    }, options.upgradeTimeout ?? 30 * 60_000)

    function finish(error?: unknown) {
      if (settled) {
        return
      }
      settled = true
      clearTimeout(timeout)
      clearTimeout(verificationTimeout)
      options.signal.removeEventListener('abort', onAbort)
      controller.abort()
      close?.()
      if (error) {
        reject(error)
      }
      else {
        options.onProgress(100)
        resolve()
      }
    }

    function onAbort() {
      finish(options.signal.reason)
    }

    options.signal.addEventListener('abort', onAbort, { once: true })

    async function start() {
      const before = await options.readVersion(signal)
      signal.throwIfAborted()
      if (!isVersionInfo(before))
        throw new Error($gettext('The node returned an invalid version response.'))
      close = options.connect({
        onConnected(socket) {
          if (settled)
            return
          try {
            socket.send(JSON.stringify(control))
          }
          catch (error) {
            finish(error)
          }
        },
        onMessage(data) {
          if (settled || verifying)
            return
          try {
            const response = JSON.parse(data)
            if (!response || !['info', 'progress', 'error'].includes(response.status))
              throw new Error($gettext('The node returned an invalid upgrade response.'))

            if (response.message)
              options.onLog(response.message)
            if (response.status === 'error') {
              finish(new Error(response.message || $gettext('The node failed to upgrade.')))
              return
            }

            // These messages are also sent by older binary and Docker nodes
            // immediately before installing or handing off to the new container.
            if (response.status === 'info' && (response.message === 'Performing core upgrade'
              || response.message === 'Docker image pull completed, upgrading...')) {
              readyToRestart = true
              progress = 90
            }
            else if (response.status === 'progress' && Number.isFinite(response.progress)) {
              progress = Math.max(progress, 10 + Math.min(100, Math.max(0, response.progress)) * 0.7)
            }
            else {
              progress = Math.min(90, progress + 5)
            }
            options.onProgress(progress)
          }
          catch (error) {
            finish(error)
          }
        },
        onError() {
          if (!readyToRestart)
            finish(new Error($gettext('The upgrade connection failed. Check authentication and network connectivity.')))
        },
        onDisconnected() {
          if (settled || verifying)
            return
          if (!readyToRestart) {
            finish(new Error($gettext('The upgrade connection closed before installation started.')))
            return
          }
          verifying = true
          if (control.dry_run) {
            finish()
            return
          }
          verificationTimeout = setTimeout(() => {
            finish(new Error($gettext('Could not verify the upgrade. Check the node version and service status.')))
          }, options.verificationTimeout ?? 300_000)
          void verifyUpgrade({ ...options, signal, target }, before).then(() => finish(), finish)
        },
      })
      if (settled)
        close()
    }
    void start().catch(finish)
  })
}
