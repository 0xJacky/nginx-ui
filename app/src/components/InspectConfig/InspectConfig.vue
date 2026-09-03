<script setup lang="ts">
import type { NgxTestResult } from '@/api/ngx'
import type { CosyError } from '@/lib/http/types'
import ngx from '@/api/ngx'
import { logLevel } from '@/constants/config'
import { translateError } from '@/lib/http/error'
import { getInspectAlertKind } from './alert'

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
const requestError = ref<string>('')
const testLoading = ref(false)

const alertKind = computed(() => getInspectAlertKind(data.value, Boolean(requestError.value), logLevel.Warn))

const statusMessage = computed(() => {
  if (data.value?.level !== undefined && data.value.level > logLevel.Warn) {
    return $gettext('Nginx configuration test failed')
  }

  switch (data.value?.sandbox_status) {
    case 'skipped':
      return $gettext('Sandbox validation skipped')
    case 'failed':
      return $gettext('Sandbox validation failed')
    default:
      return $gettext('Error')
  }
})

const sandboxReasonMessage = computed(() => {
  switch (data.value?.sandbox_reason) {
    case 'remote_namespace':
      return $gettext('Config validation is unavailable for a remote-only namespace.')
    case 'separate_container':
      return $gettext('Sandbox validation is unavailable when Nginx runs in a separate container.')
    case 'custom_test_command':
      return $gettext('Sandbox validation is unavailable because a custom test command is configured.')
    default:
      return ''
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
  requestError.value = ''
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

    requestError.value = message
    data.value = {
      ...cosyError,
      message,
      level: logLevel.Error,
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
      :title="$gettext('Testing Nginx configuration...')"
      type="info"
      show-icon
    />
    <AAlert
      v-else-if="alertKind === 'request_error'"
      :banner
      :title="$gettext('Could not reach the server')"
      type="error"
      show-icon
    >
      <template #description>
        <div v-if="translatedDetails">
          {{ translatedDetails }}
        </div>
        <div>{{ requestError }}</div>
      </template>
    </AAlert>
    <AAlert
      v-else-if="alertKind === 'skipped'"
      :banner
      :title="$gettext('Sandbox validation skipped')"
      type="info"
      show-icon
    >
      <template #description>
        <div v-if="sandboxReasonMessage">
          {{ sandboxReasonMessage }}
        </div>
        <div v-if="data?.message">
          {{ data.message }}
        </div>
      </template>
    </AAlert>
    <AAlert
      v-else-if="alertKind === 'failed'"
      :banner
      :title="$gettext('Sandbox validation failed')"
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
      v-else-if="alertKind === 'success'"
      :banner
      :title="namespaceId
        ? $gettext('Configuration file is test successful in isolated sandbox')
        : $gettext('Configuration file is test successful')"
      type="success"
      show-icon
    />
    <AAlert
      v-else-if="alertKind === 'warning'"
      :title="$gettext('Warning')"
      :banner
      type="warning"
      show-icon
    >
      <template #description>
        {{ data?.message }}
      </template>
    </AAlert>

    <AAlert
      v-else-if="alertKind === 'error'"
      :title="statusMessage"
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
