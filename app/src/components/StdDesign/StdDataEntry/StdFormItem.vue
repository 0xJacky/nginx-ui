<script setup lang="ts">
import type { Column } from '@/components/StdDesign/types'
import { computed } from 'vue'

const props = defineProps<Props>()

export interface Props {
  dataIndex?: Column['dataIndex']
  label?: string
  extra?: string
  hint?: string | (() => string)
  error?: {
    [key: string]: string
  }
  required?: boolean
}

const tag = computed(() => {
  return props.error?.[props.dataIndex!.toString()] ?? ''
})

const help = computed(() => {
  if (tag.value.includes('required'))
    return $gettext('This field should not be empty')

  return props.hint
})
</script>

<template>
  <AFormItem
    :name="dataIndex as string"
    :label="label"
    :help="help"
    :required="required"
  >
    <slot />
  </AFormItem>
</template>

<style scoped lang="less">
</style>
