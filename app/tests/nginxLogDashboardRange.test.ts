import { describe, expect, test } from 'bun:test'
import dayjs from 'dayjs'
import { browserTimeZone, buildDashboardRequest, getDefaultDashboardDateRange } from '@/views/nginx_log/dashboard/dateRange'

describe('buildDashboardRequest', () => {
  test('sends the local days together with the zone they belong to', () => {
    const range = getDefaultDashboardDateRange(dayjs('2026-11-20T00:59:55'))

    expect(buildDashboardRequest('/var/log/nginx/access.log', range, 'Asia/Shanghai')).toEqual({
      log_path: '/var/log/nginx/access.log',
      start_date: '2026-11-20',
      end_date: '2026-11-20',
      timezone: 'Asia/Shanghai',
    })
  })

  test('defaults to the browser time zone', () => {
    const range = getDefaultDashboardDateRange(dayjs('2026-11-20T12:00:00'))

    expect(buildDashboardRequest('', range).timezone).toBe(browserTimeZone())
  })
})
