import type { CosyError } from './types'
import { http, service, useAxios } from '@uozi-admin/request'
import dayjs from 'dayjs'
import JSEncrypt from 'jsencrypt'
import { storeToRefs } from 'pinia'
import use2FAModal, { TwoFACancelledError } from '@/components/TwoFA/use2FAModal'
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
          // No 2FA-protected JSON endpoint currently uses `crypto: true`
          // (login/install are pre-auth), so the original payload doesn't
          // need to be snapshotted for retry. Keeping a plaintext copy on
          // the config would also leak credentials through `console.error`.
          config.data = await encryptJsonData(config.data)
        }
      }
      // Handle form data with encryption — restore-backup is the one caller
      // that combines `crypto: true` with `RequireSecureSession()`, so a 2FA
      // retry must re-encrypt the original FormData rather than the already-
      // encrypted blob from the first attempt.
      else if (config.crypto && config.data instanceof FormData) {
        if (config._retriedAfter2FA && config._preEncryptionFormData) {
          config.data = config._preEncryptionFormData
        }
        else {
          config._preEncryptionFormData = config.data
        }
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
      const { token } = storeToRefs(user)
      const otpModal = use2FAModal()

      // Handle authentication errors
      if (error?.response) {
        switch (error.response.status) {
          case 401: {
            const failedConfig = error.config
            // A 401 from the API means the secure session expired (the only
            // middleware that returns 401 is RequireSecureSession). Re-prompt
            // for 2FA and retry once. The retry flag prevents an endless 2FA
            // loop if the replayed request keeps failing.
            if (failedConfig && !failedConfig._retriedAfter2FA) {
              let newSecureSessionId: string | undefined
              try {
                newSecureSessionId = await otpModal.open()
              }
              catch (twoFAError) {
                if (twoFAError instanceof TwoFACancelledError) {
                  // User dismissed the 2FA modal. Reject with a sanitized
                  // CosyError so caller `.catch(r => message.error(r.message))`
                  // does NOT re-surface the backend's misleading
                  // "Secure Session ID is invalid" text. An empty message
                  // means the caller's localized "Failed to ..." stays clean.
                  const cancelledError: CosyError = {
                    scope: 'auth',
                    code: 'two_factor_cancelled',
                    message: '',
                  }
                  return Promise.reject(cancelledError)
                }
                // A preflight HTTP call inside `otpModal.open()` failed
                // (network / 5xx). Its own response interceptor already
                // toasted the underlying error; surface that real cause
                // to the caller instead of the misleading original 401.
                return Promise.reject(twoFAError)
              }
              // The modal resolves with an empty string when 2FA isn't
              // enabled; without a fresh session id the retry would 401
              // again, so fall through to the original error handling.
              if (newSecureSessionId) {
                failedConfig._retriedAfter2FA = true
                // Returning the retry promise lets axios resolve the
                // original caller with the replayed response. The request
                // interceptor will attach the new X-Secure-Session-ID
                // header automatically from the user store.
                return service.request(failedConfig)
              }
            }
            break
          }
          case 403:
            if (!error.config?.skipAuthRedirect && token.value) {
              user.logout()
              await router.push('/login')
              // Reject so callers `await api.x()` don't resolve with
              // `undefined` and crash on downstream property access while
              // the route navigation is still in flight.
              const authFailedError: CosyError = error.response?.data ?? {
                code: error?.code || 'AUTH_FAILED',
                message: error?.message || 'Authorization failed',
              }
              return Promise.reject(authFailedError)
            }
            break
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
