import type { MessageInstance } from 'antdv-next/dist/message/interface'
import type { HookAPI } from 'antdv-next/dist/modal/useModal/types'
import type { NotificationInstance } from 'antdv-next/dist/notification/interface'

export const useAppStore = defineStore('app', () => {
  const message = ref<MessageInstance>()
  const modal = ref<HookAPI>()
  const notification = ref<NotificationInstance>()

  function setAppContext(context: {
    message: MessageInstance
    modal: HookAPI
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
