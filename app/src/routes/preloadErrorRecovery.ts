import version from '@/version.json'

const recoveryStorageKey = 'nginx-ui:preload-error-recovery'
const recoveryHistoryKey = '__nginxUiPreloadErrorRecovery'
const currentBuild = `${version.version}:${version.build_id}:${version.total_build}`

interface PreloadErrorRecoveryTarget {
  addEventListener: (type: string, listener: (event: Event) => void) => void
  history: Pick<History, 'state' | 'replaceState'>
  location: Pick<Location, 'reload'>
  sessionStorage: Pick<Storage, 'getItem' | 'setItem'>
}

function markRecoveryAttempt(target: PreloadErrorRecoveryTarget) {
  try {
    if (target.sessionStorage.getItem(recoveryStorageKey) === currentBuild)
      return false

    target.sessionStorage.setItem(recoveryStorageKey, currentBuild)
    return true
  }
  catch {
    // Some privacy modes deny sessionStorage; history state survives a reload too.
  }

  try {
    const state = target.history.state
    if (state?.[recoveryHistoryKey] === currentBuild)
      return false

    const nextState = state && typeof state === 'object' && !Array.isArray(state)
      ? { ...state, [recoveryHistoryKey]: currentBuild }
      : { [recoveryHistoryKey]: currentBuild }
    target.history.replaceState(nextState, '')
    return true
  }
  catch {
    return false
  }
}

export function installPreloadErrorRecovery(target: PreloadErrorRecoveryTarget = window) {
  let reloadRequested = false

  target.addEventListener('vite:preloadError', (event: Event) => {
    event.preventDefault()

    if (reloadRequested)
      return

    if (!markRecoveryAttempt(target))
      return

    reloadRequested = true
    target.location.reload()
  })
}
