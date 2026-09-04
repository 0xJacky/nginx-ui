import type { CosyError } from './types'

function asRecord(value: unknown): Record<string, unknown> | undefined {
  return value !== null && typeof value === 'object' && !Array.isArray(value)
    ? value as Record<string, unknown>
    : undefined
}

export function shouldLogoutOnAuthFailure(error: unknown, token: string): boolean {
  const failure = asRecord(error)
  const response = asRecord(failure?.response)
  const config = asRecord(failure?.config)
  const headers = asRecord(config?.headers)
  const requestToken = typeof headers?.get === 'function'
    ? headers.get('Authorization')
    : headers?.Authorization ?? headers?.authorization

  // Late failures from a previous login must not invalidate its replacement.
  return response?.status === 403 && !!token && config?.skipAuthRedirect !== true && requestToken === token
}

export function normalizeHttpError(error: unknown): CosyError {
  const failure = asRecord(error)
  const response = asRecord(failure?.response)
  const body = asRecord(response?.data)
  const status = response?.status
  const httpStatus = typeof status === 'number' && Number.isInteger(status) && status >= 100 && status <= 599
    ? status
    : undefined

  // Never use an HTML proxy error page as the message or discard the HTTP status.
  const message = typeof body?.message === 'string'
    ? body.message
    : httpStatus
      ? 'Server error'
      : typeof failure?.message === 'string' ? failure.message : 'Network error'
  const code = typeof body?.code === 'string' || typeof body?.code === 'number'
    ? body.code
    : typeof failure?.code === 'string' ? failure.code : httpStatus ? `HTTP_${httpStatus}` : 'NETWORK_ERROR'

  return {
    // Keep API-specific validation details used by form callers.
    ...body,
    code,
    message,
    httpStatus,
    scope: typeof body?.scope === 'string' ? body.scope : undefined,
    params: Array.isArray(body?.params) ? body.params.map(String) : undefined,
  }
}
