<script setup lang="ts">
import type { MessageInstance } from 'antdv-next/dist/message/interface'
import type { HookAPI } from 'antdv-next/dist/modal/useModal/types'
import type { NotificationInstance } from 'antdv-next/dist/notification/interface'
import { App } from 'antdv-next'
import { useAppStore } from '@/pinia'

const appStore = useAppStore()

// Initialize App context when this component is mounted (within AApp context)
onMounted(() => {
  try {
    const appInstance = App.useApp()
    appStore.setAppContext({
      message: appInstance.message as MessageInstance,
      modal: appInstance.modal as HookAPI,
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
