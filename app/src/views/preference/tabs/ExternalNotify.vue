<script setup lang="ts">
import type { ExternalNotify } from '@/api/external_notify'
import { StdCurd } from '@uozi-admin/curd'
import { Button, message } from 'ant-design-vue'
import externalNotify, { testMessage } from '@/api/external_notify'
import columns from '../components/ExternalNotify/columns'

const loadingStates = ref<Record<number, boolean>>({})

async function handleTestSingleMessage(record: ExternalNotify) {
  if (!record.id)
    return

  if (!record.enabled) {
    message.warning($gettext('This notification is disabled'))
    return
  }

  loadingStates.value[record.id] = true
  try {
    // Use new API with direct parameters instead of ID
    await testMessage({
      type: record.type,
      language: record.language,
      config: record.config,
    })
    message.success($gettext('Test message sent successfully'))
  }
  catch (error) {
    console.error('Test message error:', error)
    message.error($gettext('Failed to send test message'))
  }
  finally {
    loadingStates.value[record.id] = false
  }
}
</script>

<template>
  <StdCurd
    :title="$gettext('External Notify')"
    :columns="columns"
    :api="externalNotify"
    disable-view
    disable-export
    disable-trash
    disable-search
  >
    <template #beforeActions="{ record }">
      <Button
        type="link"
        size="small"
        :loading="loadingStates[record.id] || false"
        @click="handleTestSingleMessage(record)"
      >
        {{ $gettext('Test') }}
      </Button>
    </template>
  </StdCurd>
</template>

<style scoped lang="less"></style>
