import version from '@/version.json'

const recoveryStorageKey = 'nginx-ui:preload-error-recovery'
const currentBuild = `${version.version}:${version.build_id}:${version.total_build}`

interface PreloadErrorRecoveryTarget {
  addEventListener: (type: string, listener: (event: Event) => void) => void
  location: Pick<Location, 'reload'>
  sessionStorage: Pick<Storage, 'getItem' | 'setItem'>
}

export function installPreloadErrorRecovery(target: PreloadErrorRecoveryTarget = window) {
  let reloadRequested = false

  target.addEventListener('vite:preloadError', (event: Event) => {
    event.preventDefault()

    if (reloadRequested)
      return

    try {
      if (target.sessionStorage.getItem(recoveryStorageKey) === currentBuild)
        return

      target.sessionStorage.setItem(recoveryStorageKey, currentBuild)
    }
    catch {
      // Reload once even when browser privacy settings make sessionStorage unavailable.
    }

    reloadRequested = true
    target.location.reload()
  })
}
