<script setup lang="ts">
import ngx from '@/api/ngx'
import { logLevel } from '@/views/config/constants'

const data = ref({
  level: 0,
  message: '',
})

test()

function test() {
  ngx.test().then(r => {
    data.value = r
  })
}

defineExpose({
  test,
})
</script>

<template>
  <div class="inspect-container">
    <AAlert
      v-if="data?.level < logLevel.Debug"
      :message="$gettext('Configuration file is test successful')"
      type="success"
      show-icon
    />
    <AAlert
      v-else-if="data?.level === logLevel.Warn"
      :message="$gettext('Warning')"
      type="warning"
      show-icon
    >
      <template #description>
        {{ data.message }}
      </template>
    </AAlert>

    <AAlert
      v-else-if="data?.level > logLevel.Warn"
      :message="$gettext('Error')"
      type="error"
      show-icon
    >
      <template #description>
        {{ data.message }}
      </template>
    </AAlert>
  </div>
</template>

<style lang="less" scoped>
.inspect-container {
  margin-bottom: 20px;
}

:deep(.ant-alert-description) {
  white-space: pre-line;
}
</style>
