<script setup lang="ts">
import type { Settings } from '@/api/settings'

const data: Ref<Settings> = inject('data') as Ref<Settings>
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
