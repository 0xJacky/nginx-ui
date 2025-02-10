<script setup lang="ts">
import type { Variable } from '@/api/template'
import { useSettingsStore } from '@/pinia'
import _ from 'lodash'
import { storeToRefs } from 'pinia'

const data = defineModel<Variable>({
  default: () => {},
})

const { language } = storeToRefs(useSettingsStore())

const trans_name = computed(() => {
  return data.value?.name?.[language.value] ?? data.value?.name?.en ?? ''
})

const build_template = inject('build_template') as () => void

const value = computed(() => data.value.value)

watch(value, _.throttle(build_template, 500))

const selectOptions = computed(() => {
  return Object.keys(data.value?.mask || {}).map(k => {
    const label = data.value.mask?.[k]?.[language.value] ?? data.value.mask?.[k]?.en ?? ''

    return {
      label,
      value: k,
    }
  })
})
</script>

<template>
  <AFormItem :label="trans_name">
    <AInput
      v-if="data.type === 'string'"
      v-model:value="data.value"
    />
    <ASelect
      v-else-if="data.type === 'select'"
      v-model:value="data.value"
      :options="selectOptions"
    />
    <ASwitch
      v-else-if="data.type === 'boolean'"
      v-model:checked="data.value"
    />
  </AFormItem>
</template>

<style lang="less" scoped>

</style>
