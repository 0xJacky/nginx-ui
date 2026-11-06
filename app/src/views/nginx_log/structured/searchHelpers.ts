import type { StructuredTimeRange } from './timeRange'
import dayjs from 'dayjs'

export interface TimePreset {
  key: string
  label: () => string
  value: () => StructuredTimeRange
}

function lastPreset(key: string, label: () => string, amount: number, unit: dayjs.ManipulateType): TimePreset {
  return {
    key,
    label,
    value: () => ({
      start: dayjs().subtract(amount, unit),
      end: dayjs(),
    }),
  }
}

// Time range presets of the structured viewers (Grafana-style)
export const timePresets: TimePreset[] = [
  lastPreset('last-15-minutes', () => $gettext('Last 15 minutes'), 15, 'minute'),
  lastPreset('last-30-minutes', () => $gettext('Last 30 minutes'), 30, 'minute'),
  lastPreset('last-hour', () => $gettext('Last hour'), 1, 'hour'),
  lastPreset('last-4-hours', () => $gettext('Last 4 hours'), 4, 'hour'),
  lastPreset('last-12-hours', () => $gettext('Last 12 hours'), 12, 'hour'),
  lastPreset('last-24-hours', () => $gettext('Last 24 hours'), 24, 'hour'),
  lastPreset('last-7-days', () => $gettext('Last 7 days'), 7, 'day'),
  lastPreset('last-30-days', () => $gettext('Last 30 days'), 30, 'day'),
]

/** The range a preset of the Quick Select menu stands for, as of now. */
export function timePresetRange(key: string): StructuredTimeRange | undefined {
  return timePresets.find(preset => preset.key === key)?.value()
}

/**
 * Whether a request failed because the log path is not allowed or does not
 * exist (nginx_log error codes 50013-50015). Those failures stay silent.
 */
export function isPathValidationError(error: unknown): boolean {
  if (!(error instanceof Error))
    return false

  try {
    const errorData = JSON.parse(error.message)
    if (errorData.scope === 'nginx_log' && errorData.code) {
      const code = Number.parseInt(errorData.code)
      return code === 50013 || code === 50014 || code === 50015
    }
  }
  catch {
    // Not a structured error
  }

  return false
}
