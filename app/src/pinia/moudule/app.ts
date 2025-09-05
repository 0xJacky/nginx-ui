import type { MessageInstance } from 'ant-design-vue/es/message/interface'
import type { ModalStaticFunctions } from 'ant-design-vue/es/modal/confirm'
import type { NotificationInstance } from 'ant-design-vue/es/notification/interface'

export const useAppStore = defineStore('app', () => {
  const message = ref<MessageInstance>()
  const modal = ref<ModalStaticFunctions>()
  const notification = ref<NotificationInstance>()

  function setAppContext(context: {
    message: MessageInstance
    modal: ModalStaticFunctions
    notification: NotificationInstance
  }) {
    message.value = context.message
    modal.value = context.modal
    notification.value = context.notification
  }

  function clearAppContext() {
    message.value = undefined
    modal.value = undefined
    notification.value = undefined
  }

  return {
    message: readonly(message),
    modal: readonly(modal),
    notification: readonly(notification),
    setAppContext,
    clearAppContext,
  }
})
