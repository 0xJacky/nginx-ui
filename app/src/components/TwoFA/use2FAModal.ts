import twoFA from '@/api/2fa'
import Authorization from '@/components/TwoFA/Authorization.vue'
import { useAppStore, useUserStore } from '@/pinia'

function use2FAModal() {
  const app = useAppStore()
  const { modal } = storeToRefs(app)
  const refOTPAuthorization = ref<typeof Authorization>()
  // eslint-disable-next-line sonarjs/pseudo-random
  const randomId = Math.random().toString(36).substring(2, 8)
  const { secureSessionId } = storeToRefs(useUserStore())

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
          const verifyOTP = (passcode: string, recovery: string) => {
            twoFA.start_secure_session_by_otp(passcode, recovery).then(async r => {
              modalInstance.destroy()
              secureSessionId.value = r.session_id
              resolve(r.session_id)
            }).catch(async () => {
              refOTPAuthorization.value?.clearInput()
              await message.error($gettext('Invalid passcode or recovery code'))
            })
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
