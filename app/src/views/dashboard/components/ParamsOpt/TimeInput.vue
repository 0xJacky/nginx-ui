<script setup lang="ts">
import type { SelectProps } from 'antdv-next'

const modelValue = defineModel<string>()

const timeUnitOptions: SelectProps['options'] = [
  { value: 'ms', label: 'ms' },
  { value: 's', label: 's' },
  { value: 'm', label: 'm' },
  { value: 'h', label: 'h' },
  { value: 'd', label: 'd' },
  { value: 'w', label: 'w' },
  { value: 'M', label: 'M' },
  { value: 'y', label: 'y' },
]

const numberValue = ref<number>(0)
const unitValue = ref<string>('s')

watch(modelValue, val => {
  if (!val) {
    numberValue.value = 0
    unitValue.value = 's'
    return
  }
  const match = val.match(/^(\d+)([msdhwMy])$/)
  if (match) {
    numberValue.value = Number.parseInt(match[1])
    unitValue.value = match[2]
  }
}, { immediate: true })

watch(() => [numberValue.value, unitValue.value], () => {
  if (numberValue.value === 0) {
    modelValue.value = ''
    return
  }
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
    <ASelect v-model:value="unitValue" :options="timeUnitOptions" class="w-15" />
  </ASpaceCompact>
</template>
