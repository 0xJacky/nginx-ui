<script setup lang="ts">
import TemplateFormItem from '@/views/domain/ngx_conf/config_template/TemplateFormItem.vue'
import type { Variable } from '@/api/template'

const props = defineProps<{
  data: {
    [key: string]: Variable
  }
}>()

const emit = defineEmits<{
  'update:data': [data: {
    [key: string]: Variable
  }]
}>()

const data = computed({
  get() {
    return props.data
  },
  set(v) {
    emit('update:data', v)
  },
})
</script>

<template>
  <AForm layout="vertical">
    <TemplateFormItem
      v-for="(_, k) in data"
      :key="k"
      v-model:data="data[k]"
      :name="k.toString()"
    />
  </AForm>
</template>

<style lang="less" scoped>

</style>
