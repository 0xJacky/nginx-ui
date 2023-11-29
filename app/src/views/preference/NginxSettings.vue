<script setup lang="ts">
import { useGettext } from 'vue3-gettext'
import { inject } from 'vue'
import type { IData } from '@/views/preference/typedef'

const { $gettext } = useGettext()

const data: IData = inject('data')!
</script>

<template>
  <AForm layout="vertical">
    <AFormItem :label="$gettext('Nginx Access Log Path')">
      <AInput v-model:value="data.nginx.access_log_path" />
    </AFormItem>
    <AFormItem :label="$gettext('Nginx Error Log Path')">
      <AInput v-model:value="data.nginx.error_log_path" />
    </AFormItem>
  </AForm>
</template>

<style lang="less" scoped>

</style>
