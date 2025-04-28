import type { CosyError } from './types'
import use2FAModal from '@/components/TwoFA/use2FAModal'
import { useNProgress } from '@/lib/nprogress/nprogress'
import { useSettingsStore, useUserStore } from '@/pinia'
import router from '@/routes'
import JSEncrypt from 'jsencrypt'
import { storeToRefs } from 'pinia'
import { http, instance } from './client'
import { handleApiError, useMessageDedupe } from './error'

// Setup stores and refs
const user = useUserStore()
const settings = useSettingsStore()
const { token, secureSessionId } = storeToRefs(user)
const nprogress = useNProgress()
const dedupe = useMessageDedupe()

// Helper function for encrypting JSON data
// eslint-disable-next-line ts/no-explicit-any
async function encryptJsonData(data: any): Promise<string> {
  const cryptoParams = await http.get('/crypto/public_key')
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
  const cryptoParams = await http.get('/crypto/public_key')
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
  instance.interceptors.response.use(
    response => {
      nprogress.done()

      // Check if full response is requested in config
      if (response?.config?.returnFullResponse) {
        return Promise.resolve(response)
      }
      return Promise.resolve(response.data)
    },

    async error => {
      nprogress.done()
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

      const err = error.response?.data as CosyError
      handleApiError(err, dedupe)

      return Promise.reject(error.response?.data)
    },
  )
}

export function setupInterceptors() {
  setupRequestInterceptor()
  setupResponseInterceptor()
}
