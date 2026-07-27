<script setup lang="ts">
import type { HostDiagnosis, NginxDiscovery, VerifyResult } from '@/api/host_setup'
import { computed, ref, watch } from 'vue'
import hostSetup from '@/api/host_setup'
import { getErrorMessage } from '@/lib/http'
import CheckResults from '../CheckResults.vue'
import { hasBlockingFailure, toCheckRows } from '../checks'
import { applyHostDiagnosis, hasDiagnosisChanges } from '../diagnosis'
import { useHostSetupWizard } from '../useHostSetupWizard'

const { isVerificationPassed, params } = useHostSetupWizard()

const result = ref<VerifyResult | null>(null)
const discovery = ref<NginxDiscovery | null>(null)
const diagnosis = ref<HostDiagnosis | null>(null)
const hasPendingDiagnosis = ref(false)
const discoveryWarnings = ref<string[]>([])
const runError = ref('')
const running = ref(false)
// A run invalidated by an edit must not re-arm the Save button when it lands.
let runID = 0

const rows = computed(() => toCheckRows(result.value))

// The only check that validates the host's own configuration rather than the
// SSH setup, so a failure here is offered an explicit override.
const nginxTestFailed = computed(() =>
  rows.value.some(row => row.key === 'nginx_test' && row.level === 'error'),
)

function resetResult() {
  runID++
  isVerificationPassed.value = false
  result.value = null
  discovery.value = null
  diagnosis.value = null
  hasPendingDiagnosis.value = false
  discoveryWarnings.value = []
  runError.value = ''
}

async function run(skipNginxT = false) {
  resetResult()
  const currentRun = ++runID
  running.value = true
  try {
    const report = await hostSetup.diagnose(params.value)
    if (currentRun !== runID)
      return
    diagnosis.value = report
    discovery.value = report.nginx ?? null
    discoveryWarnings.value = report.warnings ?? []
    // A deliberate override must still be verifiable, so a difference is only
    // reported. The checks below test the values actually configured.
    hasPendingDiagnosis.value = hasDiagnosisChanges(params.value, report)
    // No group filter here: the final step runs the whole pipeline.
    const verified = await hostSetup.verify(params.value, { skipNginxT })
    if (currentRun !== runID)
      return
    result.value = verified
    isVerificationPassed.value = !hasBlockingFailure(toCheckRows(verified))
  }
  catch (error) {
    if (currentRun !== runID)
      return
    result.value = null
    runError.value = getErrorMessage(error)
  }
  finally {
    running.value = false
  }
}

function applyDetectedSettings() {
  if (!diagnosis.value)
    return
  applyHostDiagnosis(params.value, diagnosis.value)
  hasPendingDiagnosis.value = false
  result.value = null
  isVerificationPassed.value = false
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
        {{ $gettext('Every check validates the SSH setup itself, except nginx -t, which validates the configuration already present on the host.') }}
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
      v-if="discoveryWarnings.length"
      type="warning"
      show-icon
      :message="$gettext('Target diagnosis completed with warnings')"
    >
      <template #description>
        <ul class="m-0 pl-4">
          <li v-for="warning in discoveryWarnings" :key="warning">
            {{ warning }}
          </li>
        </ul>
      </template>
    </AAlert>

    <AAlert
      v-if="hasPendingDiagnosis"
      type="info"
      show-icon
      :message="$gettext('Detected target settings differ from the current configuration')"
    >
      <template #description>
        <p class="mb-2">
          {{ $gettext('The checks below ran against the values you configured. Keep them, or adopt the detected ones and run verification again.') }}
        </p>
        <AButton size="small" type="primary" ghost @click="applyDetectedSettings">
          {{ $gettext('Apply detected settings') }}
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

    <ADescriptions v-if="diagnosis" bordered size="small" :column="1">
      <ADescriptionsItem :label="$gettext('Operating system')">
        {{ diagnosis.os }} {{ diagnosis.arch || '' }}
      </ADescriptionsItem>
      <ADescriptionsItem :label="$gettext('Service manager')">
        {{ diagnosis.service_manager || '-' }}
      </ADescriptionsItem>
      <ADescriptionsItem v-if="diagnosis.homebrew_prefix" :label="$gettext('Homebrew prefix')">
        {{ diagnosis.homebrew_prefix }}
      </ADescriptionsItem>
    </ADescriptions>

    <ADescriptions v-if="discovery" bordered size="small" :column="1">
      <ADescriptionsItem :label="$gettext('Detected nginx version')">
        {{ discovery.version }}
      </ADescriptionsItem>
      <ADescriptionsItem :label="$gettext('nginx executable path')">
        {{ discovery.executable_path }}
      </ADescriptionsItem>
      <ADescriptionsItem :label="$gettext('Main configuration file')">
        {{ discovery.config_path || '-' }}
      </ADescriptionsItem>
      <ADescriptionsItem :label="$gettext('nginx log directory')">
        {{ discovery.log_dir || '-' }}
      </ADescriptionsItem>
      <ADescriptionsItem :label="$gettext('nginx PID file')">
        {{ discovery.pid_path || '-' }}
      </ADescriptionsItem>
      <ADescriptionsItem v-if="discovery.document_root" :label="$gettext('Homebrew document root')">
        {{ discovery.document_root }}
      </ADescriptionsItem>
    </ADescriptions>

    <CheckResults :rows="rows" />

    <AEmpty
      v-if="!result && !running && !runError"
      :description="$gettext('A run re-reads the host, opens an SSH session, checks every path from inside this container, checks sudo access, and runs nginx -t.')"
    />
  </div>
</template>
