<script setup lang="ts">
import useSystemSettingsStore from '../store'

const systemSettingsStore = useSystemSettingsStore()
const { data } = storeToRefs(systemSettingsStore)
</script>

<template>
  <AForm layout="vertical">
    <AFormItem :label="$gettext('Terminal Start Command')">
      <p>{{ data.terminal.start_cmd }}</p>
    </AFormItem>
  </AForm>
</template>

<style lang="less" scoped>

</style>
