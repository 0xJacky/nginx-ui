<script setup lang="ts">
import SensitiveString from '@/components/SensitiveString'
import useSystemSettingsStore from '../store'

const systemSettingsStore = useSystemSettingsStore()
const { data } = storeToRefs(systemSettingsStore)
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
