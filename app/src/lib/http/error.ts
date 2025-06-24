import type { CosyError, CosyErrorRecord } from './types'
import { message } from 'ant-design-vue'

const errors: Record<string, CosyErrorRecord> = {}

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
        message.error(content, duration)
      }
    },
  }
}

// Synchronous version for already registered errors
function translateErrorSync(err: CosyError): string {
  const msg = errors?.[err.scope ?? '']?.[err.code ?? '']

  if (msg) {
    // if err has params
    if (err?.params && err.params.length > 0) {
      let res = msg()

      err.params.forEach((param, index) => {
        res = res.replaceAll(`{${index}}`, param)
      })

      return res
    }
    else {
      return msg()
    }
  }
  else {
    return $gettext(err?.message ?? 'Server error')
  }
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
    return $gettext(err?.message ?? 'Server error')
  }
}

export async function handleApiError(err: CosyError, dedupe: MessageDedupe) {
  dedupe.error(await translateError(err))
}
