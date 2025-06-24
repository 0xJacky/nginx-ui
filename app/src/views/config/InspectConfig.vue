<script setup lang="ts">
import type { CosyError } from '@/lib/http/types'
import ngx from '@/api/ngx'
import { translateError } from '@/lib/http/error'
import { logLevel } from '@/views/config/constants'

defineProps<{
  banner?: boolean
}>()

interface TestResult extends CosyError {
  message: string
  level: number
}

const data = ref<TestResult>()
const translatedError = ref<string>('')

test()

function test() {
  ngx.test().then(r => {
    data.value = r
    if (r && r.level > logLevel.Warn) {
      const cosyError: CosyError = {
        ...r,
      }
      translateError(cosyError).then(translated => {
        translatedError.value = translated
      })
    }
  })
}

defineExpose({
  test,
})
</script>

<template>
  <div class="inspect-container">
    <AAlert
      v-if="data && data.level <= logLevel.Info"
      :banner
      :message="$gettext('Configuration file is test successful')"
      type="success"
      show-icon
    />
    <AAlert
      v-else-if="data?.level === logLevel.Warn"
      :message="$gettext('Warning')"
      :banner
      type="warning"
      show-icon
    >
      <template #description>
        {{ data?.message }}
      </template>
    </AAlert>

    <AAlert
      v-else-if="data && data.level > logLevel.Warn"
      :message="$gettext('Error')"
      :banner
      type="error"
      show-icon
    >
      <template #description>
        {{ translatedError }}
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

:deep(.ant-alert-banner) {
  padding: 8px 24px;
}
</style>
