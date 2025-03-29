import type { HttpConfig } from './types'
import axios from 'axios'

const instance = axios.create({
  baseURL: import.meta.env.VITE_API_ROOT,
  timeout: 50000,
  headers: { 'Content-Type': 'application/json' },
})

const http = {
  // eslint-disable-next-line ts/no-explicit-any
  get<T = any>(url: string, config: HttpConfig = {}) {
    // eslint-disable-next-line ts/no-explicit-any
    return instance.get<any, T>(url, config)
  },
  // eslint-disable-next-line ts/no-explicit-any
  post<T = any>(url: string, data: any = undefined, config: HttpConfig = {}) {
    // eslint-disable-next-line ts/no-explicit-any
    return instance.post<any, T>(url, data, config)
  },
  // eslint-disable-next-line ts/no-explicit-any
  put<T = any>(url: string, data: any = undefined, config: HttpConfig = {}) {
    // eslint-disable-next-line ts/no-explicit-any
    return instance.put<any, T>(url, data, config)
  },
  // eslint-disable-next-line ts/no-explicit-any
  delete<T = any>(url: string, config: HttpConfig = {}) {
    // eslint-disable-next-line ts/no-explicit-any
    return instance.delete<any, T>(url, config)
  },
  // eslint-disable-next-line ts/no-explicit-any
  patch<T = any>(url: string, config: HttpConfig = {}) {
    // eslint-disable-next-line ts/no-explicit-any
    return instance.patch<any, T>(url, config)
  },
}

export { http, instance }
