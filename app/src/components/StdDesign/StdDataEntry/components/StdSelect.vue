<script setup lang="ts">
import { computed, ref } from 'vue'
import type { SelectProps } from 'ant-design-vue'

const props = defineProps<{
  value: string
  mask: Record<string, string | (() => string)>
}>()

const emit = defineEmits(['update:value'])

const options = computed(() => {
  const _options = ref<SelectProps['options']>([])

  for (const [key, value] of Object.entries(props.mask)) {
    const v = value as () => string

    _options.value!.push({ label: v?.(), value: key })
  }

  return _options
})

const _value = computed({
  get() {
    let v

    if (typeof props.mask?.[props.value] === 'function')
      v = (props.mask[props.value] as () => string)()
    else if (typeof props.mask?.[props.value] === 'string')
      v = props.mask[props.value]
    else
      v = props.value

    return v
  },
  set(v) {
    emit('update:value', v)
  },
})
</script>

<template>
  <ASelect
    v-model:value="_value"
    :options="options.value"
    style="min-width: 180px"
  />
</template>

<style lang="less" scoped>

</style>
