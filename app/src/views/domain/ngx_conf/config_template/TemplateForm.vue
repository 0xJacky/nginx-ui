<script setup lang="ts">
import TemplateFormItem from '@/views/domain/ngx_conf/config_template/TemplateFormItem.vue'

const props = defineProps(['data'])
</script>

<template>
  <a-form layout="vertical">
    <template-form-item v-for="(v,k) in data" :name="k" :data="v"/>
  </a-form>
</template>

<style lang="less" scoped>

</style>
