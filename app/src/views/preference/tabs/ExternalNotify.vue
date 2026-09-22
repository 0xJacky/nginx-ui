<script setup lang="ts">
import type { ExternalNotify } from '@/api/external_notify'
import { StdCurd } from '@uozi-admin/curd'
import externalNotify, { createExternalNotify, testMessage } from '@/api/external_notify'
import columns from '../components/ExternalNotify/columns'
import { loadPluginChannels, sanitizeConfig } from '../components/ExternalNotify/pluginChannels'

const { message } = App.useApp()
const table = useTemplateRef('table')

const loadingStates = ref<Record<number, boolean>>({})
const copyLoadingStates = ref<Record<number, boolean>>({})

onMounted(() => loadPluginChannels(true))

async function handleTestSingleMessage(record: ExternalNotify) {
  if (!record.id)
    return

  if (!record.enabled) {
    message.warning($gettext('This notification is disabled'))
    return
  }

  loadingStates.value[record.id] = true
  try {
    const sanitizedConfig = sanitizeConfig(record.type, record.config)

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

async function handleCopy(record: ExternalNotify) {
  if (!record.id)
    return

  copyLoadingStates.value[record.id] = true
  try {
    const sanitizedConfig = sanitizeConfig(record.type, record.config)

    await createExternalNotify({
      type: record.type,
      description: record.description,
      language: record.language,
      config: sanitizedConfig,
      enabled: record.enabled,
    })

    message.success($gettext('Copied'))
    table.value?.refresh()
  }
  catch (error) {
    console.error('Copy external notify error:', error)
    message.error($gettext('Failed'))
  }
  finally {
    copyLoadingStates.value[record.id] = false
  }
}
</script>

<template>
  <StdCurd
    ref="table"
    hide-title
    :columns="columns"
    :api="externalNotify"
    :custom-query-params="{
      sort_by: 'id',
      order: 'asc',
    }"
    disable-view
    disable-export
    disable-trash
    disable-search
  >
    <template #beforeActions="{ record }">
      <AButton
        type="link"
        size="small"
        :loading="copyLoadingStates[record.id] || false"
        @click="handleCopy(record)"
      >
        {{ $gettext('Copy') }}
      </AButton>
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
