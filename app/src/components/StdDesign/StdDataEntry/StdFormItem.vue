<script setup lang="ts">
import { computed } from 'vue'
import { useGettext } from 'vue3-gettext'

const props = defineProps<Props>()

const { $gettext } = useGettext()

export interface Props {
  dataIndex?: string
  label?: string
  extra?: string
  error?: {
    [key: string]: string
  }
}

const tag = computed(() => {
  return props.error?.[props.dataIndex] ?? ''
})

const valid_status = computed(() => {
  if (tag.value)
    return 'error'
  else
    return 'success'
})

const help = computed(() => {
  if (tag.value.includes('required'))
    return () => $gettext('This field should not be empty')

  return () => {
  }
})
</script>

<template>
  <AFormItem
    :label="label"
    :extra="extra"
    :validate-status="valid_status"
    :help="help?.()"
  >
    <slot />
  </AFormItem>
</template>

<style scoped lang="less">

</style>
