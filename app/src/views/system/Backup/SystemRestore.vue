<script setup lang="ts">
import SystemRestoreContent from '@/components/SystemRestore/SystemRestoreContent.vue'
import { message } from 'ant-design-vue'

function handleRestoreSuccess(options: { restoreNginx: boolean, restoreNginxUI: boolean }): void {
  message.success($gettext('System restored successfully.'))

  // Only redirect to login page if Nginx UI was restored
  if (options.restoreNginxUI) {
    message.info($gettext('Please log in.'))
    window.location.reload()
  }
}
</script>

<template>
  <SystemRestoreContent :show-title="true" @restore-success="handleRestoreSuccess" />
</template>
