export interface InspectAlertResult {
  level: number
  sandbox_status?: 'ok' | 'skipped' | 'failed'
}

export type InspectAlertKind = 'request_error' | 'skipped' | 'failed' | 'success' | 'warning' | 'error' | 'none'

export function getInspectAlertKind(data: InspectAlertResult | undefined, hasRequestError: boolean, warnLevel: number): InspectAlertKind {
  if (hasRequestError) {
    return 'request_error'
  }
  if (!data) {
    return 'none'
  }
  if (data.sandbox_status === 'skipped' && data.level <= warnLevel) {
    return 'skipped'
  }
  if (data.sandbox_status === 'failed') {
    return 'failed'
  }
  if (data.level < warnLevel) {
    return 'success'
  }
  if (data.level === warnLevel) {
    return 'warning'
  }
  return 'error'
}
