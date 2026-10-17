import type { CosyError, CosyErrorRecord, HttpConfig } from './types'
import { getErrorMessage, isTwoFactorCancelled, registerError, resolveErrorMessage, useMessageDedupe } from './error'

// Export everything needed from this module
export type {
  CosyError,
  CosyErrorRecord,
  HttpConfig,
}

export {
  getErrorMessage,
  isTwoFactorCancelled,
  registerError,
  resolveErrorMessage,
  useMessageDedupe,
}
