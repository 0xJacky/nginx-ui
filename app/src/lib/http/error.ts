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

export function handleApiError(err: CosyError, dedupe: MessageDedupe) {
  if (err?.scope) {
    // check if already register
    if (!errors[err.scope]) {
      try {
        // Dynamic import error files
        import(/* @vite-ignore */ `@/constants/errors/${err.scope}.ts`)
          .then(error => {
            registerError(err.scope!, error.default)
            displayErrorMessage(err, dedupe)
          })
          .catch(() => {
            dedupe.error($gettext(err?.message ?? 'Server error'))
          })
      }
      catch {
        dedupe.error($gettext(err?.message ?? 'Server error'))
      }
    }
    else {
      displayErrorMessage(err, dedupe)
    }
  }
  else {
    dedupe.error($gettext(err?.message ?? 'Server error'))
  }
}

function displayErrorMessage(err: CosyError, dedupe: MessageDedupe) {
  const msg = errors?.[err.scope ?? '']?.[err.code ?? '']

  if (msg) {
    // if err has params
    if (err?.params && err.params.length > 0) {
      let res = msg()

      err.params.forEach((param, index) => {
        res = res.replaceAll(`{${index}}`, param)
      })

      dedupe.error(res)
    }
    else {
      dedupe.error(msg())
    }
  }
  else {
    dedupe.error($gettext(err?.message ?? 'Server error'))
  }
}
