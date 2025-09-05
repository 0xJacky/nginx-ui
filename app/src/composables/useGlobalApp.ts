import { message as antMessage, notification as antNotification, Modal } from 'ant-design-vue'
import { useAppStore } from '@/pinia'

/**
 * Global composable for Ant Design Vue App context
 * Provides message, modal, and notification APIs with fallback
 */
export function useGlobalApp() {
  const appStore = useAppStore()

  const { message, modal, notification } = storeToRefs(appStore)

  return {
    message: readonly(message.value || antMessage),
    modal: readonly(modal.value || Modal),
    notification: readonly(notification.value || antNotification),
  }
}

/**
 * Legacy compatibility - mimics App.useApp() behavior
 * @deprecated Use useGlobalApp() instead
 */
export function useApp() {
  return useGlobalApp()
}
