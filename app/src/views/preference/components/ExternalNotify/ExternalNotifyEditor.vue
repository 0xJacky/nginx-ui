<script setup lang="ts">
import type { StdTableColumn } from '@uozi-admin/curd'
import type { ExternalNotifyConfig } from './types'
import { StdForm } from '@uozi-admin/curd'
import configMap from './index'

const props = defineProps<{
  type?: string
}>()

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
</script>

<template>
  <StdForm
    v-if="currentConfig"
    v-model:data="modelValue"
    :columns
  />
</template>

<style scoped lang="less">

</style>
