import twoFA from '@/api/2fa'
import Authorization from '@/components/TwoFA/Authorization.vue'
import { useUserStore } from '@/pinia'
import { message, Modal } from 'ant-design-vue'
import { createVNode, render } from 'vue'

function use2FAModal() {
  const refOTPAuthorization = ref<typeof Authorization>()
  const randomId = Math.random().toString(36).substring(2, 8)
  const { secureSessionId } = storeToRefs(useUserStore())

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
      let container: HTMLDivElement | null = document.createElement('div')
      document.body.appendChild(container)

      const close = () => {
        render(null, container!)
        document.body.removeChild(container!)
        container = null
      }

      const setSessionId = (sessionId: string) => {
        close()
        secureSessionId.value = sessionId
        resolve(sessionId)
      }

      const verifyOTP = (passcode: string, recovery: string) => {
        twoFA.start_secure_session_by_otp(passcode, recovery).then(async r => {
          setSessionId(r.session_id)
        }).catch(async () => {
          refOTPAuthorization.value?.clearInput()
          await message.error($gettext('Invalid passcode or recovery code'))
        })
      }

      const vnode = createVNode(Modal, {
        open: true,
        title: $gettext('Two-factor authentication required'),
        centered: true,
        maskClosable: false,
        class: randomId,
        footer: false,
        onCancel: () => {
          close()
          // eslint-disable-next-line prefer-promise-reject-errors
          reject()
        },
      }, {
        default: () => h(
          Authorization,
          {
            ref: refOTPAuthorization,
            twoFAStatus,
            class: 'mt-3',
            onSubmitOTP: verifyOTP,
            onSubmitSecureSessionID: setSessionId,
          },
        ),
      })

      render(vnode, container!)
    })
  }

  return { open }
}

export default use2FAModal
