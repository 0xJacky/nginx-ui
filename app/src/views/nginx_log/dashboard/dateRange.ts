import type { DashboardRequest } from '@/api/nginx_log'
import dayjs from 'dayjs'

export function getDefaultDashboardDateRange(referenceTime = dayjs()): [dayjs.Dayjs, dayjs.Dayjs] {
  const endTime = referenceTime.endOf('day')
  return [
    endTime.startOf('day'),
    endTime,
  ]
}

// The browser's IANA time zone, or undefined when the runtime cannot name it.
export function browserTimeZone(): string | undefined {
  try {
    return new Intl.DateTimeFormat().resolvedOptions().timeZone || undefined
  }
  catch {
    return undefined
  }
}

// The date range is picked as local calendar days, so the server has to read
// the dates in the same time zone. Without it the server falls back to UTC
// days, and a browser east of UTC asks for a day that has not started yet.
export function buildDashboardRequest(
  logPath: string,
  dateRange: [dayjs.Dayjs, dayjs.Dayjs],
  timeZone = browserTimeZone(),
): DashboardRequest {
  return {
    log_path: logPath,
    start_date: dateRange[0].format('YYYY-MM-DD'),
    end_date: dateRange[1].format('YYYY-MM-DD'),
    timezone: timeZone,
  }
}
