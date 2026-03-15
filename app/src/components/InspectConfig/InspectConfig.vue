<script setup lang="ts">
import type { NgxTestResult } from '@/api/ngx'
import type { CosyError } from '@/lib/http/types'
import ngx from '@/api/ngx'
import { logLevel } from '@/constants/config'
import { translateError } from '@/lib/http/error'

const props = defineProps<{
  banner?: boolean
  namespaceId?: number | string
}>()

interface TestResult extends NgxTestResult {
  code?: string
  scope?: string
  params?: string[]
}

const data = ref<TestResult>()
const translatedError = ref<string>('')
const testLoading = ref(false)

const statusMessage = computed(() => {
  switch (data.value?.sandbox_status) {
    case 'skipped':
      return $gettext('Sandbox validation skipped')
    case 'failed':
      return $gettext('Sandbox validation failed')
    default:
      return $gettext('Error')
  }
})

const categoryMessage = computed(() => {
  switch (data.value?.error_category) {
    case 'missing_include':
      return $gettext('A required include file is missing from the sandbox or source configuration.')
    case 'sandbox_build_error':
      return $gettext('Sandbox setup failed before Nginx could validate the configuration.')
    case 'syntax_error':
      return $gettext('Nginx reported a configuration syntax error.')
    case 'nginx_runtime_error':
      return $gettext('Nginx failed to validate the configuration.')
    default:
      return ''
  }
})

const translatedDetails = computed(() => {
  if (!translatedError.value || translatedError.value === data.value?.message) {
    return ''
  }

  return translatedError.value
})

// Watch for namespace changes and auto-test
watch(() => props.namespaceId, () => {
  test()
}, { immediate: true })

async function test() {
  testLoading.value = true
  translatedError.value = ''
  const namespaceIdNum = props.namespaceId ? Number(props.namespaceId) : 0

  try {
    const result = namespaceIdNum > 0
      ? await ngx.test_namespace(namespaceIdNum)
      : await ngx.test()

    data.value = result

    const testResult = result as TestResult
    if (testResult.level > logLevel.Warn && testResult.code && testResult.scope) {
      translatedError.value = await translateError(testResult as CosyError)
    }
  }
  catch (error) {
    const cosyError = error as Partial<CosyError>
    const message = cosyError?.message ?? $gettext('Server error')

    data.value = {
      ...cosyError,
      message,
      level: logLevel.Error,
      sandbox_status: namespaceIdNum > 0 ? 'failed' : undefined,
      error_category: 'nginx_runtime_error',
      test_scope: namespaceIdNum > 0 ? 'namespace_sandbox' : 'global',
    }

    if (cosyError?.code && cosyError?.scope) {
      translatedError.value = await translateError(cosyError as CosyError)
    }
  }
  finally {
    testLoading.value = false
  }
}

defineExpose({
  test,
})
</script>

<template>
  <div class="inspect-container">
    <AAlert
      v-if="testLoading"
      :banner
      :message="$gettext('Testing Nginx configuration...')"
      type="info"
      show-icon
    />
    <AAlert
      v-else-if="data?.sandbox_status === 'skipped'"
      :banner
      :message="$gettext('Sandbox validation skipped')"
      type="info"
      show-icon
    >
      <template #description>
        {{ data?.message }}
      </template>
    </AAlert>
    <AAlert
      v-else-if="data?.sandbox_status === 'failed'"
      :banner
      :message="$gettext('Sandbox validation failed')"
      type="error"
      show-icon
    >
      <template #description>
        <div v-if="categoryMessage">
          {{ categoryMessage }}
        </div>
        <div v-if="translatedDetails">
          {{ translatedDetails }}
        </div>
        <div v-if="data?.message">
          {{ data?.message }}
        </div>
      </template>
    </AAlert>
    <AAlert
      v-else-if="data && data.level <= logLevel.Info"
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
      :message="statusMessage"
      :banner
      type="error"
      show-icon
    >
      <template #description>
        <div v-if="categoryMessage">
          {{ categoryMessage }}
        </div>
        <div v-if="translatedDetails">
          {{ translatedDetails }}
        </div>
        <div v-if="data?.message">
          {{ data?.message }}
        </div>
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
