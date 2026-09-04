import { describe, expect, mock, spyOn, test } from 'bun:test'
import { computed, ref } from 'vue'

Object.assign(globalThis, { ref, computed, $gettext: (message: string) => message })
const detailStatus = mock(() => Promise.reject(new Error('temporary failure')))
mock.module('../src/api/ngx', () => ({ default: { detail_status: detailStatus } }))
mock.module('../src/lib/helper', () => ({ formatDateTime: (value: string) => value }))
const { useNginxPerformance } = await import('../src/composables/useNginxPerformance')

const healthy = {
  running: true,
  stub_status_enabled: true,
  info: { active: 4 } as Parameters<ReturnType<typeof useNginxPerformance>['applyPerformanceData']>[0]['info'],
}

describe('Nginx performance state recovery', () => {
  test('healthy WebSocket data clears an earlier HTTP failure', async () => {
    const state = useNginxPerformance()
    const errorLog = spyOn(console, 'error').mockImplementation(() => undefined)
    try {
      await state.fetchInitialData()
      expect(errorLog).toHaveBeenCalledTimes(1)
    }
    finally {
      errorLog.mockRestore()
    }
    expect(state.error.value).toBe('Failed to get performance data')
    expect(state.nginxInfo.value).toBeNull()
    state.applyPerformanceData(healthy)
    expect(state.error.value).toBe('')
    expect(state.nginxInfo.value?.active).toBe(4)
    expect(state.formattedUpdateTime.value).not.toBe('Unknown')
    expect(state.loading.value).toBe(false)
  })

  test('server-reported errors survive until a healthy update', () => {
    const state = useNginxPerformance()
    state.applyPerformanceData({ ...healthy, error: 'stub_status unavailable' })
    expect(state.error.value).toBe('stub_status unavailable')
    state.applyPerformanceData({ ...healthy, error: '' })
    expect(state.error.value).toBe('')
  })

  test('stopped Nginx clears obsolete metrics, and recovery clears the error', () => {
    const state = useNginxPerformance()
    state.applyPerformanceData(healthy)
    state.applyPerformanceData({ ...healthy, running: false, stub_status_enabled: false })
    expect(state.nginxInfo.value).toBeNull()
    expect(state.error.value).toBe('Nginx is not running')
    expect(state.stubStatusEnabled.value).toBe(false)
    state.applyPerformanceData(healthy)
    expect(state.error.value).toBe('')
  })
})
