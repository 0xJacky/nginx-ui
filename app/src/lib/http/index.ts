import type { CosyError, CosyErrorRecord, HttpConfig } from './types'
import { registerError, useMessageDedupe } from './error'

// Export everything needed from this module
export type {
  CosyError,
  CosyErrorRecord,
  HttpConfig,
}

export {
  registerError,
  useMessageDedupe,
}
