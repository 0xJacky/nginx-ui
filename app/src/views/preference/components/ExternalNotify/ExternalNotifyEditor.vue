<script setup lang="ts">
import type { Column } from '@/components/StdDesign/types'
import type { ExternalNotifyConfig } from './types'
import StdDataEntry, { input } from '@/components/StdDesign/StdDataEntry'
import configMap from './index'

const props = defineProps<{
  type?: string
}>()

const modelValue = defineModel<Record<string, string>>({ default: reactive({}) })

const currentConfig = computed<ExternalNotifyConfig | undefined>(() => {
  return configMap[props.type?.toLowerCase() ?? '']
})

const columns = computed<Column[]>(() => {
  if (!currentConfig.value)
    return []

  return currentConfig.value.config.map(item => ({
    title: item.label,
    dataIndex: item.key,
    key: item.key,
    edit: {
      type: input,
      config: {
        label: item.label,
        required: true,
      },
    },
  }))
})
</script>

<template>
  <StdDataEntry
    v-if="currentConfig"
    v-model:data-source="modelValue"
    :data-list="columns"
  />
</template>

<style scoped lang="less">

</style>
