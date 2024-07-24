<script setup lang="ts">
import type { Settings } from '@/views/preference/typedef'

const data: Settings = inject('data')!
const errors: Record<string, Record<string, string>> = inject('errors') as Record<string, Record<string, string>>
</script>

<template>
  <AForm layout="vertical">
    <AFormItem
      :label="$gettext('Nginx Access Log Path')"
      :validate-status="errors?.nginx?.access_log_path ? 'error' : ''"
      :help="errors?.nginx?.access_log_path === 'file'
        ? $gettext('File not found')
        : ''"
    >
      <AInput v-model:value="data.nginx.access_log_path" />
    </AFormItem>
    <AFormItem
      :label="$gettext('Nginx Error Log Path')"
      :validate-status="errors?.nginx?.error_log_path ? 'error' : ''"
      :help="errors?.nginx?.error_log_path === 'file'
        ? $gettext('File not found')
        : ''"
    >
      <AInput v-model:value="data.nginx.error_log_path" />
    </AFormItem>
  </AForm>
</template>

<style lang="less" scoped>

</style>
