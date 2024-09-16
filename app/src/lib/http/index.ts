import type { AxiosRequestConfig } from 'axios'
import axios from 'axios'
import { useCookies } from '@vueuse/integrations/useCookies'
import { storeToRefs } from 'pinia'
import NProgress from 'nprogress'
import { useSettingsStore, useUserStore } from '@/pinia'
import 'nprogress/nprogress.css'

import router from '@/routes'
import use2FAModal from '@/components/2FA/use2FAModal'

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

instance.interceptors.request.use(
  config => {
    NProgress.start()
    if (token.value) {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      (config.headers as any).Authorization = token.value
    }

    if (settings.environment.id) {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      (config.headers as any)['X-Node-ID'] = settings.environment.id
    }

    if (secureSessionId.value) {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
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
    NProgress.done()

    return Promise.resolve(response.data)
  },
  async error => {
    NProgress.done()

    const otpModal = use2FAModal()
    const cookies = useCookies(['nginx-ui-2fa'])
    switch (error.response.status) {
      case 401:
        cookies.remove('secure_session_id')
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
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    return instance.get<any, any>(url, config)
  },
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  post(url: string, data: any = undefined, config: AxiosRequestConfig = {}) {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    return instance.post<any, any>(url, data, config)
  },
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  put(url: string, data: any = undefined, config: AxiosRequestConfig = {}) {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    return instance.put<any, any>(url, data, config)
  },
  delete(url: string, config: AxiosRequestConfig = {}) {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    return instance.delete<any, any>(url, config)
  },
  patch(url: string, config: AxiosRequestConfig = {}) {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    return instance.patch<any, any>(url, config)
  },
}

export default http
