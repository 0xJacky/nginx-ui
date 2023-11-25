<script setup lang="ts">
import {computed} from 'vue'
import {useGettext} from 'vue3-gettext'

const {$gettext} = useGettext()

export interface Props {
  dataIndex?: string
  label?: string
  extra?: string
  error?: any
}

const props = defineProps<Props>()

const tag = computed(() => {
  return props.error?.[props.dataIndex] ?? ''
})

const valid_status = computed(() => {
  if (!!tag.value) {
    return 'error'
  } else {
    return 'success'
  }
})

const help = computed(() => {
  if (tag.value.indexOf('required') > -1) {
    return () => $gettext('This field should not be empty')
  }
  return () => {
  }
})
</script>

<template>
  <a-form-item :label="label" :extra="extra" :validate-status="valid_status" :help="help?.()">
    <slot/>
  </a-form-item>
</template>

<style scoped lang="less">

</style>
