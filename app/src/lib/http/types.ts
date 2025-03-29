import type { AxiosRequestConfig } from 'axios'

// server response
export interface CosyError {
  scope?: string
  code: string
  message: string
  params?: string[]
}

// code, message translation
export type CosyErrorRecord = Record<number, () => string>

export interface HttpConfig extends AxiosRequestConfig {
  returnFullResponse?: boolean
  crypto?: boolean
}

// Extend InternalAxiosRequestConfig type
declare module 'axios' {
  interface InternalAxiosRequestConfig {
    returnFullResponse?: boolean
    crypto?: boolean
  }
}
