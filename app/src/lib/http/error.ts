import type { CosyError, CosyErrorRecord } from './types'

const errors: Record<string, CosyErrorRecord> = {}

interface ErrorResponse {
  response?: {
    data?: unknown
  }
}

// Cosy sends the template and its arguments separately, so a raw message still
// contains {0} style placeholders.
function substituteParams(message: string, params?: string[]): string {
  if (!params?.length)
    return message
  return params.reduce((result, param, index) => result.replaceAll(`{${index}}`, param), message)
}

// The interceptor rejects a dismissed 2FA prompt with an empty message on
// purpose. Reporting a fallback here would surface a bogus server error.
export function isTwoFactorCancelled(error: unknown): boolean {
  if (!error || typeof error !== 'object')
    return false
  return (error as CosyError).code === 'two_factor_cancelled'
}

function resolveCosyMessage(candidate: unknown): string {
  if (!candidate || typeof candidate !== 'object')
    return ''
  const err = candidate as CosyError
  if (typeof err.message !== 'string' || !err.message)
    return ''
  return translateErrorSync(err)
}

export function getErrorMessage(error: unknown, fallback = $gettext('Server error')): string {
  if (typeof error === 'string' && error)
    return error

  if (!error || typeof error !== 'object')
    return fallback

  if (isTwoFactorCancelled(error) || isTwoFactorCancelled((error as ErrorResponse).response?.data))
    return ''

  const errorResponse = error as ErrorResponse
  return resolveCosyMessage(errorResponse.response?.data)
    || resolveCosyMessage(error)
    || fallback
}

export function registerError(scope: string, record: CosyErrorRecord) {
  errors[scope] = record
}

// Add new dedupe utility
interface MessageDedupe {
  error: (content: string, duration?: number) => void
}

export function useMessageDedupe(interval = 5000): MessageDedupe {
  const lastMessages = new Map<string, number>()

  return {
    async error(content, duration = 5) {
      const now = Date.now()
      if (!lastMessages.has(content) || (now - (lastMessages.get(content) || 0)) > interval) {
        lastMessages.set(content, now)

        // Use global App context with fallback
        const { message } = useGlobalApp()
        message.error(content, duration)
      }
    },
  }
}

// Synchronous version for already registered errors
function translateErrorSync(err: CosyError): string {
  const msg = errors?.[err.scope ?? '']?.[err.code ?? '']

  if (msg)
    return substituteParams(msg(), err?.params)

  return substituteParams(fallbackMessage(err), err?.params)
}

function fallbackMessage(err: CosyError): string {
  const message = err?.message ?? 'Server error'
  const translated = $gettext(message)
  return message === 'Server error' && err.httpStatus
    ? `${translated} (HTTP ${err.httpStatus})`
    : translated
}

// Asynchronous version that handles dynamic loading
export async function translateError(err: CosyError): Promise<string> {
  // If scope exists, use sync version
  if (!err?.scope || errors[err.scope]) {
    return translateErrorSync(err)
  }

  // Need to dynamically load error definitions
  try {
    const errorModule = await import(`@/constants/errors/${err.scope}.ts`)
    registerError(err.scope, errorModule.default)
    return translateErrorSync(err)
  }
  catch (error) {
    console.error(error)
    return substituteParams(fallbackMessage(err), err?.params)
  }
}

export async function handleApiError(err: CosyError, dedupe: MessageDedupe) {
  dedupe.error(await translateError(err))
}
