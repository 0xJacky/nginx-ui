<script setup lang="ts">
import type { StdTableColumn } from '@uozi-admin/curd'
import type { ExternalNotifyConfig } from './types'
import { StdForm } from '@uozi-admin/curd'
import { testMessage } from '@/api/external_notify'
import gettext from '@/gettext'
import configMap from './index'

const props = defineProps<{
  type?: string
}>()

const { message } = App.useApp()

const modelValue = defineModel<Record<string, string>>({ default: reactive({}) })

const currentConfig = computed<ExternalNotifyConfig | undefined>(() => {
  return configMap[props.type?.toLowerCase() ?? '']
})

const columns = computed<StdTableColumn[]>(() => {
  if (!currentConfig.value)
    return []

  return currentConfig.value.config.map(item => ({
    title: item.label,
    dataIndex: item.key,
    key: item.key,
    edit: {
      type: 'input',
      formItem: {
        label: item.label,
      },
    },
  }))
})

const loading = ref(false)

async function handleSendTestMessage() {
  if (!props.type) {
    message.error($gettext('Please select a notification type'))
    return
  }

  loading.value = true
  try {
    await testMessage({
      type: props.type,
      language: gettext.current,
      config: modelValue.value,
    })
    message.success($gettext('Test message sent successfully'))
  }
  catch (error) {
    console.error('Test message error:', error)
  }
  finally {
    loading.value = false
  }
}
</script>

<template>
  <div v-if="currentConfig">
    <StdForm
      v-model:data="modelValue"
      :columns
    />

    <div>
      <AButton
        type="primary"
        size="small"
        :loading="loading"
        @click="handleSendTestMessage"
      >
        {{ $gettext("Send test message") }}
      </AButton>
    </div>
  </div>
</template>

<style scoped lang="less">

</style>
