<script setup lang="ts">
import type { Variable } from '@/api/template'
import { useSettingsStore } from '@/pinia'
import { storeToRefs } from 'pinia'
import { useConfigTemplateStore } from './store'

const data = defineModel<Variable>({
  default: reactive({}),
})

const { language } = storeToRefs(useSettingsStore())

const configTemplateStore = useConfigTemplateStore()

const transName = computed(() => {
  return data.value?.name?.[language.value] ?? data.value?.name?.en ?? ''
})

const value = computed(() => data.value.value)

watch(value, configTemplateStore.buildTemplate)

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
  <AFormItem :label="transName">
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
