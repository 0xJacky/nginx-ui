<script setup lang="ts">
import type { CosyError } from '@/lib/http/types'
import ngx from '@/api/ngx'
import { logLevel } from '@/constants/config'
import { translateError } from '@/lib/http/error'

const props = defineProps<{
  banner?: boolean
  namespaceId?: number | string
}>()

interface TestResult extends CosyError {
  message: string
  level: number
  namespace_id?: number
}

const data = ref<TestResult>()
const translatedError = ref<string>('')
const testLoading = ref(false)

// Watch for namespace changes and auto-test
watch(() => props.namespaceId, () => {
  test()
}, { immediate: true })

function test() {
  testLoading.value = true
  const namespaceIdNum = props.namespaceId ? Number(props.namespaceId) : 0
  const testPromise = namespaceIdNum > 0
    ? ngx.test_namespace(namespaceIdNum)
    : ngx.test()

  testPromise.then(r => {
    data.value = r
    if (r && r.level > logLevel.Warn) {
      const cosyError: CosyError = {
        ...r,
      }
      translateError(cosyError).then(translated => {
        translatedError.value = translated
      })
    }
  }).finally(() => {
    testLoading.value = false
  })
}

defineExpose({
  test,
})
</script>

<template>
  <div class="inspect-container">
    <!-- Test Results -->
    <AAlert
      v-if="data && data.level <= logLevel.Info"
      :banner
      :message="namespaceId
        ? $gettext('Configuration file is test successful in isolated sandbox')
        : $gettext('Configuration file is test successful')"
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
