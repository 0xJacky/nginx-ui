import { describe, expect, mock, test } from 'bun:test'
import { installPreloadErrorRecovery } from '../src/routes/preloadErrorRecovery'

interface FixtureOptions {
  historyState?: { value: unknown }
  initialMarker?: string
  throwOnStorage?: boolean
}

function fixture(options: FixtureOptions = {}) {
  const events = new EventTarget()
  const values = new Map<string, string>()
  const historyState = options.historyState ?? { value: null }
  if (options.initialMarker)
    values.set('nginx-ui:preload-error-recovery', options.initialMarker)

  const reload = mock(() => {})
  installPreloadErrorRecovery({
    addEventListener: events.addEventListener.bind(events),
    history: {
      get state() {
        return historyState.value
      },
      replaceState: value => {
        historyState.value = value
      },
    },
    location: { reload },
    sessionStorage: {
      getItem: key => {
        if (options.throwOnStorage)
          throw new Error('storage denied')
        return values.get(key) ?? null
      },
      setItem: (key, value) => {
        if (options.throwOnStorage)
          throw new Error('storage denied')
        values.set(key, value)
      },
    },
  })

  return { events, historyState, reload, values }
}

describe('preload error recovery', () => {
  test('prevents the failed import and reloads once per page instance', () => {
    const { events, reload, values } = fixture()
    const first = new Event('vite:preloadError', { cancelable: true })
    const second = new Event('vite:preloadError', { cancelable: true })

    events.dispatchEvent(first)
    events.dispatchEvent(second)

    expect(first.defaultPrevented).toBe(true)
    expect(second.defaultPrevented).toBe(false)
    expect(reload).toHaveBeenCalledTimes(1)
    expect(values.get('nginx-ui:preload-error-recovery')).toBeTruthy()
  })

  test('does not loop when the current build already attempted recovery', () => {
    const first = fixture()
    first.events.dispatchEvent(new Event('vite:preloadError', { cancelable: true }))
    const currentMarker = first.values.get('nginx-ui:preload-error-recovery')

    const second = fixture({ initialMarker: currentMarker })
    const event = new Event('vite:preloadError', { cancelable: true })
    second.events.dispatchEvent(event)

    expect(event.defaultPrevented).toBe(false)
    expect(second.reload).not.toHaveBeenCalled()
  })

  test('uses reload-persistent history state when session storage is denied', () => {
    const historyState = { value: null as unknown }
    const first = fixture({ historyState, throwOnStorage: true })
    first.events.dispatchEvent(new Event('vite:preloadError', { cancelable: true }))
    expect(first.reload).toHaveBeenCalledTimes(1)

    const second = fixture({ historyState, throwOnStorage: true })
    const event = new Event('vite:preloadError', { cancelable: true })
    second.events.dispatchEvent(event)

    expect(event.defaultPrevented).toBe(false)
    expect(second.reload).not.toHaveBeenCalled()
  })

  test('does not reload when no persistent guard is available', () => {
    const f = fixture({ throwOnStorage: true })
    Object.defineProperty(f.historyState, 'value', {
      get: () => null,
      set: () => { throw new Error('history denied') },
    })

    const event = new Event('vite:preloadError', { cancelable: true })
    f.events.dispatchEvent(event)

    expect(event.defaultPrevented).toBe(false)
    expect(f.reload).not.toHaveBeenCalled()
  })
})
