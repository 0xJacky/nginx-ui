import type { CosyError } from './types'
import { http, useAxios } from '@uozi-admin/request'
import dayjs from 'dayjs'
import JSEncrypt from 'jsencrypt'
import { storeToRefs } from 'pinia'
import use2FAModal from '@/components/TwoFA/use2FAModal'
import { getBrowserFingerprint } from '@/lib/helper'
import { useSettingsStore, useUserStore } from '@/pinia'
import router from '@/routes'
import { handleApiError, useMessageDedupe } from './error'

const { setRequestInterceptor, setResponseInterceptor } = useAxios()

const dedupe = useMessageDedupe()

// Helper function for encrypting JSON data
// eslint-disable-next-line ts/no-explicit-any
async function encryptJsonData(data: any): Promise<string> {
  const fingerprint = await getBrowserFingerprint()
  const cryptoParams = await http.post('/crypto/public_key', {
    timestamp: dayjs().unix(),
    fingerprint,
  })
  const { public_key } = await cryptoParams

  // Encrypt data with RSA public key
  const encrypt = new JSEncrypt()
  encrypt.setPublicKey(public_key)

  return JSON.stringify({
    encrypted_params: encrypt.encrypt(JSON.stringify(data)),
  })
}

// Helper function for handling encrypted form data
async function handleEncryptedFormData(formData: FormData): Promise<FormData> {
  const fingerprint = await getBrowserFingerprint()
  const cryptoParams = await http.post('/crypto/public_key', {
    timestamp: dayjs().unix(),
    fingerprint,
  })
  const { public_key } = await cryptoParams

  // Extract form parameters that are not files
  // eslint-disable-next-line ts/no-explicit-any
  const formParams: Record<string, any> = {}
  const newFormData = new FormData()

  // Copy all files to new FormData
  for (const [key, value] of formData.entries()) {
    // Check if value is a File or Blob
    // eslint-disable-next-line ts/no-explicit-any
    if (typeof value !== 'string' && ((value as any) instanceof File || (value as any) instanceof Blob)) {
      newFormData.append(key, value)
    }
    else {
      // Collect non-file fields to encrypt
      formParams[key] = value
    }
  }

  // Encrypt the form parameters
  const encrypt = new JSEncrypt()
  encrypt.setPublicKey(public_key)

  // Add encrypted params to form data
  const encryptedData = encrypt.encrypt(JSON.stringify(formParams))
  if (encryptedData) {
    newFormData.append('encrypted_params', encryptedData)
  }

  return newFormData
}

// Setup request interceptor
export function setupRequestInterceptor() {
  // Setup stores and refs
  const user = useUserStore()
  const settings = useSettingsStore()
  const { token, secureSessionId } = storeToRefs(user)
  setRequestInterceptor(
    async config => {
      if (token.value) {
        config.headers.Authorization = token.value
      }

      if (settings.node.id) {
        config.headers['X-Node-ID'] = settings.node.id
      }

      if (secureSessionId.value) {
        config.headers['X-Secure-Session-ID'] = secureSessionId.value
      }

      // Handle JSON encryption
      if (config.headers?.['Content-Type'] !== 'multipart/form-data;charset=UTF-8') {
        config.headers['Content-Type'] = 'application/json'

        if (config.crypto) {
          config.data = await encryptJsonData(config.data)
        }
      }
      // Handle form data with encryption
      else if (config.crypto && config.data instanceof FormData) {
        config.data = await handleEncryptedFormData(config.data)
      }

      return config
    },
    err => {
      return Promise.reject(err)
    },
  )
}

// Setup response interceptor
export function setupResponseInterceptor() {
  setResponseInterceptor(
    response => {
      // Check if full response is requested in config
      if (response?.config?.returnFullResponse) {
        return Promise.resolve(response)
      }
      return Promise.resolve(response.data)
    },

    async error => {
      // Ignore canceled requests (navigation, component unmount, deduped requests)
      if (error?.code === 'ERR_CANCELED' || /canceled/i.test(error?.message || '')) {
        return Promise.reject(error)
      }
      // Setup stores and refs
      const user = useUserStore()
      const { secureSessionId } = storeToRefs(user)
      const otpModal = use2FAModal()

      // Handle authentication errors
      if (error?.response) {
        switch (error.response.status) {
          case 401:
            secureSessionId.value = ''
            await otpModal.open()
            break
          case 403:
            user.logout()
            await router.push('/login')
            return
        }
      }

      // Handle JSON error that comes back as Blob for blob request type
      if (error?.response?.data instanceof Blob && error?.response?.data?.type === 'application/json') {
        try {
          const text = await error.response.data.text()
          error.response.data = JSON.parse(text)
        }
        catch (e) {
          // If parsing fails, we'll continue with the original error.response.data
          console.error('Failed to parse blob error response as JSON', e)
        }
      }
      console.error(error)
      const errData = (error.response?.data as CosyError) || {
        code: error?.code || 'NETWORK_ERROR',
        message: error?.message || 'Network error',
      }
      await handleApiError(errData, dedupe)

      return Promise.reject(error.response?.data ?? errData)
    },
  )
}

export function setupInterceptors() {
  setupRequestInterceptor()
  setupResponseInterceptor()
}
