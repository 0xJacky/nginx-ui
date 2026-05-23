import twoFA from '@/api/2fa'
import Authorization from '@/components/TwoFA/Authorization.vue'
import { useAppStore, useUserStore } from '@/pinia'

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

  const open = async (): Promise<string> => {
    const twoFAStatus = await twoFA.status()
    const { status: secureSessionStatus } = await twoFA.secure_session_status()

    return new Promise((resolve, reject) => {
      if (!twoFAStatus.enabled) {
        resolve('')
        return
      }

      if (secureSessionId.value && secureSessionStatus) {
        resolve(secureSessionId.value)
        return
      }

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
          // eslint-disable-next-line prefer-promise-reject-errors
          reject()
        },
      })
    })
  }

  return { open }
}

export default use2FAModal
