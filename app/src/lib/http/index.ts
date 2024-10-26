import type { AxiosRequestConfig } from 'axios'
import use2FAModal from '@/components/TwoFA/use2FAModal'
import { useNProgress } from '@/lib/nprogress/nprogress'
import { useSettingsStore, useUserStore } from '@/pinia'
import router from '@/routes'
import axios from 'axios'

import { storeToRefs } from 'pinia'
import 'nprogress/nprogress.css'

const user = useUserStore()
const settings = useSettingsStore()
const { token, secureSessionId } = storeToRefs(user)

const instance = axios.create({
  baseURL: import.meta.env.VITE_API_ROOT,
  timeout: 50000,
  headers: { 'Content-Type': 'application/json' },
  transformRequest: [function (data, headers) {
    if (!(headers) || headers['Content-Type'] === 'multipart/form-data;charset=UTF-8')
      return data
    else
      headers['Content-Type'] = 'application/json'

    return JSON.stringify(data)
  }],
})

const nprogress = useNProgress()

instance.interceptors.request.use(
  config => {
    nprogress.start()
    if (token.value) {
      // eslint-disable-next-line ts/no-explicit-any
      (config.headers as any).Authorization = token.value
    }

    if (settings.environment.id) {
      // eslint-disable-next-line ts/no-explicit-any
      (config.headers as any)['X-Node-ID'] = settings.environment.id
    }

    if (secureSessionId.value) {
      // eslint-disable-next-line ts/no-explicit-any
      (config.headers as any)['X-Secure-Session-ID'] = secureSessionId.value
    }

    return config
  },
  err => {
    return Promise.reject(err)
  },
)

instance.interceptors.response.use(
  response => {
    nprogress.done()

    return Promise.resolve(response.data)
  },
  async error => {
    nprogress.done()

    const otpModal = use2FAModal()
    switch (error.response.status) {
      case 401:
        secureSessionId.value = ''
        await otpModal.open()
        break
      case 403:
        user.logout()
        await router.push('/login')
        break
    }

    return Promise.reject(error.response.data)
  },
)

const http = {
  get(url: string, config: AxiosRequestConfig = {}) {
    // eslint-disable-next-line ts/no-explicit-any
    return instance.get<any, any>(url, config)
  },
  // eslint-disable-next-line ts/no-explicit-any
  post(url: string, data: any = undefined, config: AxiosRequestConfig = {}) {
    // eslint-disable-next-line ts/no-explicit-any
    return instance.post<any, any>(url, data, config)
  },
  // eslint-disable-next-line ts/no-explicit-any
  put(url: string, data: any = undefined, config: AxiosRequestConfig = {}) {
    // eslint-disable-next-line ts/no-explicit-any
    return instance.put<any, any>(url, data, config)
  },
  delete(url: string, config: AxiosRequestConfig = {}) {
    // eslint-disable-next-line ts/no-explicit-any
    return instance.delete<any, any>(url, config)
  },
  patch(url: string, config: AxiosRequestConfig = {}) {
    // eslint-disable-next-line ts/no-explicit-any
    return instance.patch<any, any>(url, config)
  },
}

export default http
