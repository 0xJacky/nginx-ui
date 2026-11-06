import type { ErrorLogEntry, ErrorLogLevel } from '@/api/nginx_log'

/** nginx error log severities, from least to most severe. */
export const ERROR_LOG_LEVELS: readonly ErrorLogLevel[] = [
  'debug',
  'info',
  'notice',
  'warn',
  'error',
  'crit',
  'alert',
  'emerg',
]

// Tag preset colors: they follow the theme, so they read well in dark mode too
const levelColors: Record<ErrorLogLevel, string> = {
  debug: 'default',
  info: 'blue',
  notice: 'cyan',
  warn: 'gold',
  error: 'volcano',
  crit: 'red',
  alert: 'magenta',
  emerg: 'purple',
}

/** The Tag color of a severity, neutral for one nginx does not write. */
export function errorLevelColor(level: string | undefined): string {
  return levelColors[(level ?? '').toLowerCase() as ErrorLogLevel] ?? 'default'
}

/** Options of the level select; the values are the keywords nginx writes. */
export const errorLevelOptions = ERROR_LOG_LEVELS.map(level => ({ value: level, label: level }))

/** The `level` search parameter for the selected severities, if any. */
export function errorLevelParam(levels: string[]): string | undefined {
  const known = levels
    .map(level => level.trim().toLowerCase())
    .filter(level => (ERROR_LOG_LEVELS as readonly string[]).includes(level))

  return known.length > 0
    ? Array.from(new Set(known)).join(',')
    : undefined
}

/**
 * The request an entry was logged for, as "METHOD path". Falls back to the
 * request line nginx wrote when it could not be split, empty without one.
 */
export function errorLogRequest(entry: Pick<ErrorLogEntry, 'method' | 'path' | 'request'>): string {
  const method = entry.method?.trim() ?? ''
  const path = entry.path?.trim() ?? ''
  if (method || path)
    return [method, path].filter(Boolean).join(' ')

  return entry.request?.trim() ?? ''
}

/** The secondary details of an entry: server, upstream and host, when set. */
export function errorLogContext(entry: Pick<ErrorLogEntry, 'server' | 'upstream' | 'host'>): { key: 'server' | 'upstream' | 'host', value: string }[] {
  return (['server', 'upstream', 'host'] as const)
    .map(key => ({ key, value: entry[key]?.trim() ?? '' }))
    .filter(item => item.value !== '')
}
