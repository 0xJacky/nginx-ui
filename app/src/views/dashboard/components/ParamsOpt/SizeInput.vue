<script setup lang="ts">
import type { SelectProps } from 'antdv-next'

const modelValue = defineModel<string>()

const sizeUnitOptions: SelectProps['options'] = [
  { value: 'k', label: 'K' },
  { value: 'm', label: 'M' },
  { value: 'g', label: 'G' },
]

const numberValue = ref<number>(0)
const unitValue = ref<string>('m')

watch(modelValue, val => {
  if (!val) {
    numberValue.value = 0
    unitValue.value = 'm'
    return
  }
  const match = val.match(/^(\d+)([kmg])$/)
  if (match) {
    numberValue.value = Number.parseInt(match[1])
    unitValue.value = match[2]
  }
}, { immediate: true })

watch(() => [numberValue.value, unitValue.value], () => {
  modelValue.value = `${numberValue.value}${unitValue.value}`
})
</script>

<template>
  <ASpaceCompact>
    <AInputNumber
      v-model:value="numberValue"
      :step="1"
      class="w-30"
    />
    <ASelect v-model:value="unitValue" :options="sizeUnitOptions" class="w-15" />
  </ASpaceCompact>
</template>
