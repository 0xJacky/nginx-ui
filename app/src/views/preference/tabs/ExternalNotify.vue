<script setup lang="ts">
import type { ExternalNotify } from '@/api/external_notify'
import { StdCurd } from '@uozi-admin/curd'
import externalNotify, { testMessage } from '@/api/external_notify'
import configMap from '../components/ExternalNotify'
import columns from '../components/ExternalNotify/columns'

const { message } = App.useApp()

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
    const notifierConfig = configMap[record.type?.toLowerCase() ?? '']
    const allowedConfigKeys = new Set((notifierConfig?.config ?? []).map(item => item.key))
    const sanitizedConfig = Object.fromEntries(
      Object.entries(record.config ?? {}).filter(([key]) => allowedConfigKeys.has(key)),
    )

    // Use new API with direct parameters instead of ID
    await testMessage({
      type: record.type,
      language: record.language,
      config: sanitizedConfig,
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
      <AButton
        type="link"
        size="small"
        :loading="loadingStates[record.id] || false"
        @click="handleTestSingleMessage(record)"
      >
        {{ $gettext('Test') }}
      </AButton>
    </template>
  </StdCurd>
</template>

<style scoped lang="less"></style>
