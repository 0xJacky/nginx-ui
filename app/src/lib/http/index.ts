import type { AxiosRequestConfig } from 'axios'
import use2FAModal from '@/components/TwoFA/use2FAModal'
import { useNProgress } from '@/lib/nprogress/nprogress'
import { useSettingsStore, useUserStore } from '@/pinia'
import router from '@/routes'
import { message } from 'ant-design-vue'
import axios from 'axios'
import JSEncrypt from 'jsencrypt'
import { storeToRefs } from 'pinia'
import 'nprogress/nprogress.css'

const user = useUserStore()
const settings = useSettingsStore()
const { token, secureSessionId } = storeToRefs(user)

// server response
export interface CosyError {
  scope?: string
  code: string
  message: string
  params?: string[]
}

// code, message translation
export type CosyErrorRecord = Record<number, () => string>

const errors: Record<string, CosyErrorRecord> = {}

function registerError(scope: string, record: CosyErrorRecord) {
  errors[scope] = record
}

const instance = axios.create({
  baseURL: import.meta.env.VITE_API_ROOT,
  timeout: 50000,
  headers: { 'Content-Type': 'application/json' },
})

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

const nprogress = useNProgress()

// Add new dedupe utility at the top
interface MessageDedupe {
  error: (content: string, duration?: number) => void
}

function useMessageDedupe(interval = 5000): MessageDedupe {
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

instance.interceptors.request.use(
  async config => {
    nprogress.start()
    if (token.value) {
      config.headers.Authorization = token.value
    }

    if (settings.environment.id) {
      config.headers['X-Node-ID'] = settings.environment.id
    }

    if (secureSessionId.value) {
      config.headers['X-Secure-Session-ID'] = secureSessionId.value
    }

    if (config.headers?.['Content-Type'] !== 'multipart/form-data;charset=UTF-8') {
      config.headers['Content-Type'] = 'application/json'

      if (config.crypto) {
        const cryptoParams = await http.get('/crypto/public_key')
        const { public_key } = await cryptoParams

        // Encrypt data with RSA public key
        const encrypt = new JSEncrypt()
        encrypt.setPublicKey(public_key)

        config.data = JSON.stringify({
          encrypted_params: encrypt.encrypt(JSON.stringify(config.data)),
        })
      }
    }
    return config
  },
  err => {
    return Promise.reject(err)
  },
)

const dedupe = useMessageDedupe()

instance.interceptors.response.use(
  response => {
    nprogress.done()
    return Promise.resolve(response.data)
  },
  // eslint-disable-next-line sonarjs/cognitive-complexity
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

    const err = error.response.data as CosyError

    if (err?.scope) {
      // check if already register
      if (!errors[err.scope]) {
        try {
          const error = await import(`@/constants/errors/${err.scope}.ts`)

          registerError(err.scope, error.default)
        }
        catch {
          /* empty */
        }
      }

      const msg = errors?.[err.scope]?.[err.code]

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
    else {
      dedupe.error($gettext(err?.message ?? 'Server error'))
    }

    return Promise.reject(error.response.data)
  },
)
