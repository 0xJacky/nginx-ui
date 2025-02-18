<script setup lang="ts">
import type { Column } from '@/components/StdDesign/types'
import type { Rule } from 'ant-design-vue/es/form'
import FormErrors from '@/constants/form_errors'

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
  noValidate?: boolean
}

const tag = computed(() => {
  return props.error?.[props.dataIndex!.toString()] ?? ''
})

const help = computed(() => {
  const rules = tag.value.split(',')

  for (const rule of rules) {
    if (FormErrors[rule])
      return FormErrors[rule]()
  }

  return props.hint
})

// eslint-disable-next-line ts/no-explicit-any
async function validator(_: Rule, value: any): Promise<any> {
  return new Promise((resolve, reject) => {
    if (props.required && !props.noValidate && (!value && value !== 0)) {
      reject(help.value ?? $gettext('This field should not be empty'))

      return
    }

    resolve(true)
  })
}
</script>

<template>
  <AFormItem
    :name="dataIndex as string"
    :label="label"
    :help="help"
    :rules="{ required, validator }"
    :validate-status="tag ? 'error' : undefined"
    :auto-link="false"
  >
    <slot />
  </AFormItem>
</template>

<style scoped lang="less">
</style>
