<script setup lang="ts">
import type { VerifyResult } from '@/api/host_setup'
import { computed, ref, watch } from 'vue'
import hostSetup from '@/api/host_setup'
import { getErrorMessage } from '@/lib/http'
import CheckResults from '../CheckResults.vue'
import { hasBlockingFailure, toCheckRows } from '../checks'
import { useHostSetupWizard } from '../useHostSetupWizard'

const { isVerificationPassed, params } = useHostSetupWizard()

const result = ref<VerifyResult | null>(null)
const runError = ref('')
const running = ref(false)
// A run invalidated by an edit must not re-arm the Save button when it lands.
let runID = 0

const allRows = computed(() => toCheckRows(result.value))
const rows = computed(() => allRows.value.filter(row =>
  row.key === 'nginx_test' || (row.key === 'ssh_connect' && row.level === 'error')))

// The only check that validates the host's own configuration rather than the
// SSH setup, so a failure here is offered an explicit override.
const nginxTestFailed = computed(() =>
  rows.value.some(row => row.key === 'nginx_test' && row.level === 'error'),
)

function resetResult() {
  runID++
  isVerificationPassed.value = false
  result.value = null
  runError.value = ''
}

async function run(skipNginxT = false) {
  resetResult()
  const currentRun = ++runID
  running.value = true
  try {
    const verified = await hostSetup.verify(
      { ...params.value },
      { skipNginxT, groups: ['nginx'] },
    )
    if (currentRun !== runID)
      return
    result.value = verified
    isVerificationPassed.value = !hasBlockingFailure(allRows.value)
  }
  catch (error) {
    if (currentRun !== runID)
      return
    result.value = null
    runError.value = getErrorMessage(error)
  }
  finally {
    if (currentRun === runID)
      running.value = false
  }
}

watch(params, resetResult, { deep: true })
</script>

<template>
  <div class="space-y-4">
    <ACard size="small" :title="$gettext('Verification')">
      <template #extra>
        <AButton type="primary" size="small" :loading="running" @click="run()">
          {{ $gettext('Run verification') }}
        </AButton>
      </template>
      <ATypographyText type="secondary" class="text-xs">
        {{ $gettext('The earlier steps already verified SSH, platform, file access and privileges. This final check only runs nginx -t against the host configuration.') }}
      </ATypographyText>
    </ACard>

    <AAlert
      v-if="nginxTestFailed"
      type="warning"
      show-icon
      :message="$gettext('The nginx configuration on the host failed validation')"
    >
      <template #description>
        <p class="mb-2">
          {{ $gettext('This is the configuration already present on the host, not something this wizard wrote. The SSH setup itself can still be saved, and the configuration can be fixed from Nginx UI afterwards.') }}
        </p>
        <AButton size="small" danger ghost :loading="running" @click="run(true)">
          {{ $gettext('Continue without validating the configuration') }}
        </AButton>
      </template>
    </AAlert>

    <AAlert
      v-if="runError"
      type="error"
      show-icon
      :message="$gettext('Verification failed')"
      :description="runError"
    />

    <CheckResults :rows="rows" />

    <AEmpty
      v-if="!result && !running && !runError"
      :description="$gettext('Run the final nginx configuration check to enable saving.')"
    />
  </div>
</template>
