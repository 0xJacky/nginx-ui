<script setup lang="ts">
import type { Settings } from '@/api/settings'
import SensitiveString from '@/components/SensitiveString/SensitiveString.vue'

const data: Ref<Settings> = inject('data') as Ref<Settings>
</script>

<template>
  <AForm layout="vertical">
    <AFormItem :label="$gettext('Jwt Secret')">
      <SensitiveString :value="data.app.jwt_secret" />
    </AFormItem>
    <AFormItem :label="$gettext('Page Size')">
      <p>{{ data.app.page_size }}</p>
    </AFormItem>
  </AForm>
</template>

<style lang="less" scoped>
</style>
