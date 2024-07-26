import { createVNode, render } from 'vue'
import { Modal, message } from 'ant-design-vue'
import { useCookies } from '@vueuse/integrations/useCookies'
import OTPAuthorization from '@/components/OTP/OTPAuthorization.vue'
import otp from '@/api/otp'
import { useUserStore } from '@/pinia'

const useOTPModal = () => {
  const refOTPAuthorization = ref<typeof OTPAuthorization>()
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
    const { status } = await otp.status()

    return new Promise((resolve, reject) => {
      if (!status) {
        resolve('')

        return
      }

      const cookies = useCookies(['nginx-ui-2fa'])
      const ssid = cookies.get('secure_session_id')
      if (ssid) {
        resolve(ssid)
        secureSessionId.value = ssid

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

      const verify = (passcode: string, recovery: string) => {
        otp.start_secure_session(passcode, recovery).then(r => {
          cookies.set('secure_session_id', r.session_id, { maxAge: 60 * 3 })
          resolve(r.session_id)
          close()
          secureSessionId.value = r.session_id
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
          OTPAuthorization,
          {
            ref: refOTPAuthorization,
            class: 'mt-3',
            onOnSubmit: verify,
          },
        ),
      })

      render(vnode, container!)
    })
  }

  return { open }
}

export default useOTPModal
