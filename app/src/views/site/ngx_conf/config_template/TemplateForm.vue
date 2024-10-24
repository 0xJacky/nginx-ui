<script setup lang="ts">
import type { Variable } from '@/api/template'
import TemplateFormItem from '@/views/site/ngx_conf/config_template/TemplateFormItem.vue'

const data = defineModel<Record<string, Variable>>({
  default: () => {},
})
</script>

<template>
  <AForm layout="vertical">
    <TemplateFormItem
      v-for="(_, k) in data"
      :key="k"
      v-model="data[k]"
      :name="k.toString()"
    />
  </AForm>
</template>

<style lang="less" scoped>

</style>
