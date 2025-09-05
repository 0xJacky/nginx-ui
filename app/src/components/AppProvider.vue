<script setup lang="ts">
import type { MessageInstance } from 'ant-design-vue/es/message/interface'
import type { ModalStaticFunctions } from 'ant-design-vue/es/modal/confirm'
import type { NotificationInstance } from 'ant-design-vue/es/notification/interface'
import { App } from 'ant-design-vue'
import { useAppStore } from '@/pinia'

const appStore = useAppStore()

// Initialize App context when this component is mounted (within AApp context)
onMounted(() => {
  try {
    const appInstance = App.useApp()
    appStore.setAppContext({
      message: appInstance.message as MessageInstance,
      modal: appInstance.modal as ModalStaticFunctions,
      notification: appInstance.notification as NotificationInstance,
    })
  }
  catch (error) {
    console.warn('Failed to initialize App context:', error)
  }
})

// Clean up when component is unmounted
onUnmounted(() => {
  appStore.clearAppContext()
})
</script>

<template>
  <slot />
</template>
