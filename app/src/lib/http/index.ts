import type { CosyError, CosyErrorRecord, HttpConfig } from './types'
import { http } from './client'
import { registerError, useMessageDedupe } from './error'
import { setupInterceptors } from './interceptors'

// Initialize interceptors
setupInterceptors()

// Export everything needed from this module
export default http
export type {
  CosyError,
  CosyErrorRecord,
  HttpConfig,
}
export {
  registerError,
  useMessageDedupe,
}
