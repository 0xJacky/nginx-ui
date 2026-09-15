import { describe, expect, mock, test } from 'bun:test'
import { installPreloadErrorRecovery } from '../src/routes/preloadErrorRecovery'

function fixture(initialMarker?: string) {
  const events = new EventTarget()
  const values = new Map<string, string>()
  if (initialMarker)
    values.set('nginx-ui:preload-error-recovery', initialMarker)

  const reload = mock(() => {})
  installPreloadErrorRecovery({
    addEventListener: events.addEventListener.bind(events),
    location: { reload },
    sessionStorage: {
      getItem: key => values.get(key) ?? null,
      setItem: (key, value) => values.set(key, value),
    },
  })

  return { events, reload, values }
}

describe('preload error recovery', () => {
  test('prevents the failed import and reloads once per page instance', () => {
    const { events, reload, values } = fixture()
    const first = new Event('vite:preloadError', { cancelable: true })
    const second = new Event('vite:preloadError', { cancelable: true })

    events.dispatchEvent(first)
    events.dispatchEvent(second)

    expect(first.defaultPrevented).toBe(true)
    expect(second.defaultPrevented).toBe(true)
    expect(reload).toHaveBeenCalledTimes(1)
    expect(values.get('nginx-ui:preload-error-recovery')).toBeTruthy()
  })

  test('does not loop when the current build already attempted recovery', () => {
    const first = fixture()
    first.events.dispatchEvent(new Event('vite:preloadError', { cancelable: true }))
    const currentMarker = first.values.get('nginx-ui:preload-error-recovery')

    const second = fixture(currentMarker)
    const event = new Event('vite:preloadError', { cancelable: true })
    second.events.dispatchEvent(event)

    expect(event.defaultPrevented).toBe(true)
    expect(second.reload).not.toHaveBeenCalled()
  })
})
