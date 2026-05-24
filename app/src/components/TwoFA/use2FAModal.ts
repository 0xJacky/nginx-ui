import twoFA from '@/api/2fa'
import Authorization from '@/components/TwoFA/Authorization.vue'
import { useAppStore, useUserStore } from '@/pinia'

// Thrown when the user dismisses the 2FA prompt. Callers (notably the HTTP
// response interceptor) use `instanceof` to distinguish a user cancel from
// a preflight HTTP failure so the right error reaches the original caller.
export class TwoFACancelledError extends Error {
  constructor() {
    super('Two-factor authentication cancelled')
    this.name = 'TwoFACancelledError'
  }
}

// Module-level dedup: when several concurrent requests fail with 401 at the
// same time (e.g. a dashboard mount firing parallel protected GETs after the
// secure session expired), every awaiter shares ONE 2FA prompt. The promise
// is cleared in `.finally()` so a subsequent — independent — challenge can
// spawn a fresh modal.
let inflightOpen: Promise<string> | null = null

function use2FAModal() {
  const app = useAppStore()
  const { modal } = storeToRefs(app)
  const router = useRouter()
  const userStore = useUserStore()
  const refOTPAuthorization = ref<typeof Authorization>()
  // eslint-disable-next-line sonarjs/pseudo-random
  const randomId = Math.random().toString(36).substring(2, 8)
  const { secureSessionId } = storeToRefs(userStore)

  // Use global message API
  const { message } = useGlobalApp()

  const injectStyles = () => {
    const style = document.createElement('style')

    style.innerHTML = `
      .${randomId} .ant-modal-title {
        font-size: 1.125rem;
      }
    `
    document.head.appendChild(style)
  }

  function guideLegacyRecoveryMigration() {
    modal.value!.confirm({
      title: $gettext('Generate new recovery codes'),
      content: $gettext('Your legacy recovery code has been used and cannot be used again. Generate new recovery codes now to keep account recovery available.'),
      okText: $gettext('Go to Recovery Codes'),
      cancelText: $gettext('Later'),
      centered: true,
      onOk: () => router.push('/profile'),
    })
  }

  const openInternal = async (): Promise<string> => {
    const twoFAStatus = await twoFA.status()
    const { status: secureSessionStatus } = await twoFA.secure_session_status()

    return new Promise((resolve, reject) => {
      if (!twoFAStatus.enabled) {
        resolve('')
        return
      }

      // Fast path: another flow (e.g. a sibling tab) may have refreshed the
      // session between when the caller saw a 401 and now. Don't show the
      // modal in that case — reuse the freshly-minted session id.
      if (secureSessionId.value && secureSessionStatus) {
        resolve(secureSessionId.value)
        return
      }

      // Server confirmed the session is invalid. Clear the stale value here
      // (NOT eagerly in the caller) so the fast-path above still gets a
      // chance to recover when another flow already refreshed the session.
      secureSessionId.value = ''

      injectStyles()

      // Create modal instance to be able to destroy it later
      const modalInstance = modal.value!.confirm({
        title: $gettext('Two-factor authentication required'),
        centered: true,
        maskClosable: false,
        class: randomId,
        footer: null,
        appContext: getCurrentInstance()?.appContext,
        width: '500px',
        content: () => {
          const verifyOTP = async (passcode: string, recovery: string) => {
            let result
            try {
              result = await twoFA.start_secure_session_by_otp(passcode, recovery)
            }
            catch {
              refOTPAuthorization.value?.clearInput()
              await message.error($gettext('Invalid passcode or recovery code'))
              return
            }

            modalInstance.destroy()
            secureSessionId.value = result.session_id
            resolve(result.session_id)

            try {
              await userStore.refreshTwoFAStatus()
              if (result.used_legacy_recovery_code)
                guideLegacyRecoveryMigration()
            }
            catch (error) {
              console.error('Failed to handle post-OTP 2FA refresh:', error)
            }
          }

          const setSessionId = (sessionId: string) => {
            modalInstance.destroy()
            secureSessionId.value = sessionId
            resolve(sessionId)
          }

          return h(
            Authorization,
            {
              ref: refOTPAuthorization,
              twoFAStatus,
              class: 'mt-3 mr-34px',
              onSubmitOTP: verifyOTP,
              onSubmitSecureSessionID: setSessionId,
            },
          )
        },
        onCancel: () => {
          modalInstance.destroy()
          reject(new TwoFACancelledError())
        },
      })
    })
  }

  const open = (): Promise<string> => {
    if (inflightOpen) {
      return inflightOpen
    }
    inflightOpen = openInternal().finally(() => {
      inflightOpen = null
    })
    return inflightOpen
  }

  return { open }
}

export default use2FAModal
