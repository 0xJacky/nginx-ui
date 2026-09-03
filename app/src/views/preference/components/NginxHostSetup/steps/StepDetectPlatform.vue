<script setup lang="ts">
import type { HostDiagnosis, SetupParams } from '@/api/host_setup'
import { AimOutlined, CheckCircleOutlined, ScanOutlined, ThunderboltOutlined } from '@antdv-next/icons'
import { computed, onActivated, onDeactivated, ref, watch } from 'vue'
import hostSetup from '@/api/host_setup'
import {
  applyHostDiagnosis,
  detectedSettings,
  discoveredNginxPaths,
  hasDiagnosisChanges,
  suggestedDefaults,
} from '../diagnosis'
import PathField from '../PathField.vue'
import { useHostSetupWizard } from '../useHostSetupWizard'
import { useLatestRequest } from '../useLatestRequest'

type ServiceManager = NonNullable<SetupParams['service_manager']>
type HomebrewPrefix = '/opt/homebrew' | '/usr/local'

const { detectedValue, isPlatformReady, params, recordDetected, requestParams } = useHostSetupWizard()

const linuxPaths = {
  nginx_sbin_path: '/usr/sbin/nginx',
  host_config_dir: '/etc/nginx',
  host_log_dir: '/var/log/nginx',
  pid_path: '/var/run/nginx.pid',
}

function homebrewPaths(prefix: HomebrewPrefix) {
  return {
    nginx_sbin_path: `${prefix}/opt/nginx/bin/nginx`,
    host_config_dir: `${prefix}/etc/nginx`,
    host_log_dir: `${prefix}/var/log/nginx`,
    pid_path: `${prefix}/var/run/nginx.pid`,
  }
}

const pathKeys = ['nginx_sbin_path', 'host_config_dir', 'host_log_dir', 'pid_path'] as const
const knownPathDefaults = {
  nginx_sbin_path: new Set([
    linuxPaths.nginx_sbin_path,
    '/opt/homebrew/bin/nginx',
    '/opt/homebrew/opt/nginx/bin/nginx',
    '/usr/local/bin/nginx',
    '/usr/local/opt/nginx/bin/nginx',
  ]),
  host_config_dir: new Set([linuxPaths.host_config_dir, '/opt/homebrew/etc/nginx', '/usr/local/etc/nginx']),
  host_log_dir: new Set([linuxPaths.host_log_dir, '/opt/homebrew/var/log/nginx', '/usr/local/var/log/nginx']),
  pid_path: new Set([linuxPaths.pid_path, '/opt/homebrew/var/run/nginx.pid', '/usr/local/var/run/nginx.pid']),
}

function inferHomebrewPrefix(): HomebrewPrefix {
  const configuredPaths = [params.value.nginx_sbin_path, params.value.host_config_dir, params.value.host_log_dir]
  return configuredPaths.some(path => path?.startsWith('/usr/local/')) ? '/usr/local' : '/opt/homebrew'
}

const homebrewPrefix = ref<HomebrewPrefix>(inferHomebrewPrefix())
const diagnosis = ref<HostDiagnosis | null>(null)
// The step is cached by KeepAlive, so remember which target the report is for.
const lastDiagnosedHostAddress = ref('')
// Slow responses must not overwrite state after the step is left or a newer
// request starts.
const diagnoseRequest = useLatestRequest()
const discoverRequest = useLatestRequest()
const { error: diagnosisError, isLoading: isDiagnosing } = diagnoseRequest
const { error: discoverError, isLoading: isDiscovering } = discoverRequest
const discoverHint = ref('')
// The editable fields stay folded on the happy path. Once anything needs
// attention the panel opens and stays open, so an edit cannot scroll away.
const adjustPanels = ref<string[]>([])

function revealAdjustPanel() {
  if (!adjustPanels.value.length)
    adjustPanels.value = ['adjust']
}
function applyPathPreset(preset: ReturnType<typeof homebrewPaths> | typeof linuxPaths) {
  for (const key of pathKeys) {
    const currentValue = params.value[key]
    // A value the host reported wins over a preset, even when it happens to
    // match one of the conventional defaults.
    if (currentValue && currentValue === detectedValue(key))
      continue
    if (!currentValue || knownPathDefaults[key].has(currentValue))
      params.value[key] = preset[key]
  }
}

watch(() => params.value.service_manager, (manager: ServiceManager | undefined) => {
  if (manager === 'launchd') {
    params.value.launchd_service ||= 'homebrew.mxcl.nginx'
    params.value.launchctl_path ||= '/bin/launchctl'
    applyPathPreset(homebrewPaths(homebrewPrefix.value))
    return
  }
  params.value.systemd_unit ||= 'nginx.service'
  params.value.systemctl_path ||= '/bin/systemctl'
  applyPathPreset(linuxPaths)
})

watch(homebrewPrefix, prefix => {
  if (params.value.service_manager === 'launchd')
    applyPathPreset(homebrewPaths(prefix))
})

const hasDetectedChanges = computed(() => diagnosis.value ? hasDiagnosisChanges(params.value, diagnosis.value) : false)
const warnings = computed(() => diagnosis.value?.warnings ?? [])
const isUnclassifiedTarget = computed(() => Boolean(diagnosis.value) && !diagnosis.value?.service_manager)

/** Fill still-empty fields with conventional values the host cannot report. */
function fillSuggestedDefaults(result: HostDiagnosis) {
  for (const [key, value] of Object.entries(suggestedDefaults(result))) {
    const field = key as keyof SetupParams
    if (!params.value[field])
      Object.assign(params.value, { [field]: value })
  }
}

async function diagnoseTarget() {
  discoverError.value = ''
  discoverHint.value = ''
  diagnosis.value = null
  const target = params.value.host_address
  const isFirstDiagnosisForTarget = lastDiagnosedHostAddress.value !== target
  await diagnoseRequest.run(() => hostSetup.diagnose(requestParams.value), {
    onSuccess: result => {
      diagnosis.value = result
      lastDiagnosedHostAddress.value = target
      if (result.homebrew_prefix === '/usr/local' || result.homebrew_prefix === '/opt/homebrew')
        homebrewPrefix.value = result.homebrew_prefix
      // The first diagnosis is authoritative for this target. Later manual
      // overrides remain intact when the operator explicitly detects again.
      if (isFirstDiagnosisForTarget)
        applyHostDiagnosis(params.value, result)
      else
        fillSuggestedDefaults(result)
      recordDetected(detectedSettings(result))
    },
  })
}

function applyDetectedSettings() {
  if (!diagnosis.value)
    return
  applyHostDiagnosis(params.value, diagnosis.value)
  recordDetected(detectedSettings(diagnosis.value))
}

/** Runs nginx -V on the entered executable and refreshes the derived paths. */
async function rediscoverNginxPaths() {
  const requestedSbinPath = params.value.nginx_sbin_path
  discoverHint.value = ''
  await discoverRequest.run(() => hostSetup.discover(requestParams.value), {
    onSuccess: discovery => {
      // The executable field was edited meanwhile, so these paths describe
      // another binary.
      if (requestedSbinPath !== params.value.nginx_sbin_path)
        return
      const detected = discoveredNginxPaths(discovery)
      Object.assign(params.value, detected)
      recordDetected(detected)
      if (diagnosis.value)
        diagnosis.value = { ...diagnosis.value, nginx: discovery }
      discoverHint.value = $gettext('Paths refreshed from %{path}', { path: discovery.executable_path })
    },
  })
}

watch([isUnclassifiedTarget, diagnosisError, isPlatformReady, hasDetectedChanges], () => {
  if (isUnclassifiedTarget.value || diagnosisError.value || !isPlatformReady.value || hasDetectedChanges.value)
    revealAdjustPanel()
}, { immediate: true })

onActivated(() => {
  if (!diagnosis.value || lastDiagnosedHostAddress.value !== params.value.host_address)
    void diagnoseTarget()
})

onDeactivated(() => {
  diagnoseRequest.invalidate()
  discoverRequest.invalidate()
})
</script>

<template>
  <div class="space-y-4">
    <ACard size="small" :title="$gettext('Target diagnosis')">
      <template #extra>
        <AButton size="small" :loading="isDiagnosing" @click="diagnoseTarget">
          <ScanOutlined />
          {{ diagnosis ? $gettext('Detect again') : $gettext('Detect target') }}
        </AButton>
      </template>

      <ASpace orientation="vertical" size="middle" class="w-full">
        <ASpace v-if="diagnosis" wrap>
          <ATag v-if="!hasDetectedChanges" color="success" variant="filled">
            <CheckCircleOutlined />
            {{ $gettext('Every field matches the detected values') }}
          </ATag>
          <ATag v-else color="warning" variant="filled">
            <ThunderboltOutlined />
            {{ $gettext('Some fields differ from the detected values') }}
          </ATag>
          <AButton v-if="hasDetectedChanges" size="small" type="primary" ghost @click="applyDetectedSettings">
            <AimOutlined />
            {{ $gettext('Apply all detected values') }}
          </AButton>
        </ASpace>

        <AAlert
          v-if="diagnosisError"
          type="error"
          show-icon
          :title="$gettext('Target diagnosis failed')"
          :description="diagnosisError"
        />

        <AAlert
          v-if="isUnclassifiedTarget"
          type="warning"
          show-icon
          :title="$gettext('The target platform could not be classified automatically')"
          :description="$gettext('Choose the platform and confirm every path below manually, then continue.')"
        />

        <AAlert v-if="warnings.length" type="warning" show-icon :title="$gettext('Diagnosis completed with warnings')">
          <template #description>
            <ul class="m-0 pl-4">
              <li v-for="warning in warnings" :key="warning">
                {{ warning }}
              </li>
            </ul>
          </template>
        </AAlert>

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
          <ADescriptionsItem :label="$gettext('Detected nginx version')">
            {{ diagnosis.nginx?.version || '-' }}
          </ADescriptionsItem>
          <ADescriptionsItem :label="$gettext('nginx executable path')">
            {{ diagnosis.nginx?.executable_path || '-' }}
          </ADescriptionsItem>
        </ADescriptions>
      </ASpace>
    </ACard>

    <ACollapse
      v-model:active-key="adjustPanels"
      ghost
      :items="[{ key: 'adjust', label: $gettext('Adjust detected values') }]"
    >
      <template #contentRender>
        <ACard size="small" :title="$gettext('Platform')">
          <AFormItem :label="$gettext('Host platform')" required>
            <ASegmented
              v-model:value="params.service_manager"
              :options="[
                { label: $gettext('Linux (systemd)'), value: 'systemd' },
                { label: $gettext('macOS (Homebrew)'), value: 'launchd' },
              ]"
            />
          </AFormItem>

          <AFormItem v-if="params.service_manager === 'launchd'" :label="$gettext('Homebrew prefix')">
            <ASegmented
              v-model:value="homebrewPrefix"
              :options="[
                { label: $gettext('Apple Silicon'), value: '/opt/homebrew' },
                { label: $gettext('Intel Mac'), value: '/usr/local' },
              ]"
            />
            <template #extra>
              <ATypographyText type="secondary" class="text-xs">
                {{ $gettext('Detection fills this in. Switching it rewrites the four paths below that still hold a conventional default.') }}
              </ATypographyText>
            </template>
          </AFormItem>

          <template v-if="params.service_manager === 'systemd'">
            <PathField field="systemd_unit" :label="$gettext('systemd unit')" placeholder="nginx.service" required />
            <PathField field="systemctl_path" :label="$gettext('systemctl path')" placeholder="/bin/systemctl" required />
          </template>
          <template v-else>
            <PathField field="launchd_service" :label="$gettext('launchd service label')" placeholder="homebrew.mxcl.nginx" required />
            <PathField field="launchctl_path" :label="$gettext('launchctl path')" placeholder="/bin/launchctl" required />
          </template>
        </ACard>

        <ACard size="small" :title="$gettext('nginx paths')">
          <PathField field="nginx_sbin_path" :label="$gettext('nginx executable path')" placeholder="/usr/sbin/nginx" required />

          <AFormItem>
            <ASpace orientation="vertical" size="small" class="w-full">
              <ASpace wrap>
                <AButton :loading="isDiscovering" @click="rediscoverNginxPaths">
                  <ScanOutlined />
                  {{ $gettext('Re-detect paths from this executable') }}
                </AButton>
              </ASpace>
              <ATypographyText type="secondary" class="text-xs">
                {{ $gettext('Runs nginx -V on the target and refreshes the config, log and PID paths below.') }}
              </ATypographyText>
            </ASpace>
          </AFormItem>

          <AAlert
            v-if="discoverError"
            type="error"
            show-icon
            class="mb-4"
            :title="$gettext('Could not read the nginx build configuration')"
            :description="discoverError"
          />
          <AAlert
            v-else-if="discoverHint"
            type="success"
            show-icon
            class="mb-4"
            :title="discoverHint"
          />

          <PathField field="host_config_dir" :label="$gettext('nginx config directory')" placeholder="/etc/nginx" required />
          <PathField field="host_log_dir" :label="$gettext('nginx log directory')" placeholder="/var/log/nginx" required />
          <PathField field="pid_path" :label="$gettext('nginx PID file')" placeholder="/var/run/nginx.pid" required />
        </ACard>
      </template>
    </ACollapse>

    <AAlert
      v-if="isPlatformReady"
      type="success"
      show-icon
      :title="$gettext('Platform and paths are complete.')"
    />
  </div>
</template>
