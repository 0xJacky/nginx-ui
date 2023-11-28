<script setup lang="ts">
import { storeToRefs } from 'pinia'
import _ from 'lodash'
import { useSettingsStore } from '@/pinia'
import type { Variable } from '@/api/template'

const props = defineProps<{
  data: Variable
  name: string
}>()

const emit = defineEmits<{
  'update:data': (data: Variable) => void
}>()

const data = computed({
  get() {
    return props.data
  },
  set(v) {
    emit('update:data', v)
  },
})

const { language } = storeToRefs(useSettingsStore())

const trans_name = computed(() => {
  return props.data?.name?.[language.value] ?? props.data?.name?.en ?? ''
})

const build_template = inject('build_template') as () => void

const value = computed(() => props.data.value)

watch(value, _.throttle(build_template, 500))
</script>

<template>
  <AFormItem :label="trans_name">
    <AInput
      v-if="data.type === 'string'"
      v-model:value="data.value"
    />
    <ASwitch
      v-else-if="data.type === 'boolean'"
      v-model:checked="data.value"
    />
  </AFormItem>
</template>

<style lang="less" scoped>

</style>
