import { createVNode, render } from 'vue'
import { Modal, message } from 'ant-design-vue'
import OTPAuthorization from '@/components/OTP/OTPAuthorization.vue'
import otp from '@/api/otp'

export interface OTPModalProps {
  onOk?: (secureSessionId: string) => void
  onCancel?: () => void
}

const useOTPModal = () => {
  const refOTPAuthorization = ref<typeof OTPAuthorization>()
  const randomId = Math.random().toString(36).substring(2, 8)

  const injectStyles = () => {
    const style = document.createElement('style')

    style.innerHTML = `
      .${randomId} .ant-modal-title {
        font-size: 1.125rem;
      }
    `
    document.head.appendChild(style)
  }

  const open = ({ onOk, onCancel }: OTPModalProps) => {
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
        onOk?.(r.session_id)
        close()
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
        onCancel?.()
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

    render(vnode, container)
  }

  return { open }
}

export default useOTPModal
