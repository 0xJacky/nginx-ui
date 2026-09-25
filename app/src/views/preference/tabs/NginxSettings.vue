<script setup lang="ts">
import type { NginxControlMode, NginxSettings } from '@/api/settings'
import { ArrowRightOutlined, CloseOutlined, EditOutlined, SaveOutlined } from '@antdv-next/icons'
import settingsApi from '@/api/settings'
import { SettingPanel, SettingRow } from '@/components/SettingPanel'
import { TwoFACancelledError, use2FAModal } from '@/components/TwoFA'
import {
  applyNginxControlSettings,
  buildNginxControlPayload,
  cloneNginxSettings,
  NGINX_CONTROL_PATHS,
  resolveNginxControlMode,
} from '../nginxControl'
import useSystemSettingsStore from '../store'

const emit = defineEmits<{
  controlEditing: [value: boolean]
}>()

const systemSettingsStore = useSystemSettingsStore()
const { data } = storeToRefs(systemSettingsStore)
const { message, modal } = useGlobalApp()
const twoFAModal = use2FAModal()
const router = useRouter()

const isEditingControl = ref(false)
const isSavingControl = ref(false)
const selectedMode = ref<NginxControlMode>('local')
const containerName = ref('')
const hasContainerNameError = ref(false)
let controlSnapshot: NginxSettings | null = null

const currentMode = computed(() => resolveNginxControlMode(data.value.nginx))

function syncEditorFromSettings() {
  selectedMode.value = resolveNginxControlMode(data.value.nginx)
  containerName.value = data.value.nginx.container_name || ''
  hasContainerNameError.value = false
}

function guideToTwoFASettings() {
  let shouldOpenTwoFASettings = false

  modal.confirm({
    title: $gettext('Two-factor authentication required'),
    content: `${$gettext('User Profile')} > ${$gettext('2FA Settings')}`,
    okText: $gettext('2FA Settings'),
    cancelText: $gettext('Cancel'),
    centered: true,
    onOk: () => {
      shouldOpenTwoFASettings = true
    },
    afterClose: () => {
      if (shouldOpenTwoFASettings) {
        void router.push({
          path: '/profile',
          hash: '#two-factor-authentication',
        })
      }
    },
  })
}

watch(
  () => [data.value.nginx.host_mode, data.value.nginx.container_name],
  () => {
    if (!isEditingControl.value)
      syncEditorFromSettings()
  },
  { immediate: true },
)

watch(isEditingControl, value => {
  emit('controlEditing', value)
}, { immediate: true })

onUnmounted(() => {
  emit('controlEditing', false)
})

async function beginControlEdit() {
  try {
    const secureSessionID = await twoFAModal.open()
    if (!secureSessionID) {
      guideToTwoFASettings()
      return
    }

    controlSnapshot = cloneNginxSettings(data.value.nginx)
    syncEditorFromSettings()
    isEditingControl.value = true
  }
  catch (error) {
    if (!(error instanceof TwoFACancelledError))
      console.error('Failed to authorize nginx control settings:', error)
  }
}

function applyModeChange(value: NginxControlMode) {
  selectedMode.value = value
  hasContainerNameError.value = false
  if (value === 'host_via_ssh') {
    data.value.nginx.host_mode = 'ssh'
    data.value.nginx.container_name = ''
  }
  else if (value === 'external_container') {
    data.value.nginx.host_mode = ''
    data.value.nginx.container_name = containerName.value
  }
  else {
    data.value.nginx.host_mode = ''
    data.value.nginx.container_name = ''
  }
}

function onModeChange(value: NginxControlMode) {
  applyModeChange(value)
}

watch(containerName, value => {
  if (isEditingControl.value && selectedMode.value === 'external_container')
    data.value.nginx.container_name = value
})

async function saveControlSettings() {
  hasContainerNameError.value = selectedMode.value === 'external_container' && !containerName.value.trim()
  if (hasContainerNameError.value)
    return

  isSavingControl.value = true
  try {
    const saved = await settingsApi.saveNginxControl(
      buildNginxControlPayload(data.value.nginx, selectedMode.value, containerName.value),
    )
    applyNginxControlSettings(data.value.nginx, saved)
    systemSettingsStore.markSaved(NGINX_CONTROL_PATHS)
    selectedMode.value = saved.mode
    containerName.value = saved.container_name
    controlSnapshot = null
    isEditingControl.value = false
    message.success($gettext('Save successfully'))
  }
  catch (error) {
    console.error('Failed to save nginx control settings:', error)
  }
  finally {
    isSavingControl.value = false
  }
}

function cancelControlEdit() {
  if (controlSnapshot)
    data.value.nginx = cloneNginxSettings(controlSnapshot)

  controlSnapshot = null
  isEditingControl.value = false
  syncEditorFromSettings()
}

async function openSSHSetup() {
  if (controlSnapshot)
    data.value.nginx = cloneNginxSettings(controlSnapshot)

  controlSnapshot = null
  isEditingControl.value = false
  await router.push('/preference/nginx-host-setup')
}
</script>

<template>
  <div>
    <SettingPanel :title="$gettext('Status')">
      <SettingRow
        :title="$gettext('Stub Status Port')"
        :description="$gettext('Local port used to read Nginx connection statistics.')"
        path="nginx.stub_status_port"
      >
        <AInputNumber v-model:value="data.nginx.stub_status_port" class="w-30" />
      </SettingRow>
    </SettingPanel>

    <SettingPanel :title="$gettext('Maintenance Page')">
      <SettingRow
        :title="$gettext('Maintenance host')"
        :description="$gettext('Optional HTTP or HTTPS origin that serves the maintenance page. Leave empty to use this Nginx UI instance.')"
        path="nginx.maintenance_host"
      >
        <AInput
          v-model:value="data.nginx.maintenance_host"
          :placeholder="$gettext('http://127.0.0.1:9000')"
          class="w-60"
        />
      </SettingRow>
      <SettingRow
        :title="$gettext('Maintenance bypass IP')"
        :description="$gettext('Requests from this IPv4 or IPv6 address continue to use the original site while maintenance mode is active.')"
        path="nginx.maintenance_bypass_ip"
      >
        <AInput
          v-model:value="data.nginx.maintenance_bypass_ip"
          :placeholder="$gettext('203.0.113.10')"
          class="w-60"
        />
      </SettingRow>
      <SettingRow
        :title="$gettext('Maintenance template (filename only)')"
        path="nginx.maintenance_template"
      >
        <template #description>
          <div>
            {{ $gettext('The file named <site name>.<filename> is used first; if it does not exist, the generic <filename> is used; if neither exists, the built-in Nginx UI maintenance page is used.') }}
          </div>
          <div>
            {{ $gettext('Mounted directory') }}: {{ data.nginx.maintenance_dir }}
          </div>
        </template>
        <AInput
          v-model:value="data.nginx.maintenance_template"
          :placeholder="$gettext('maintenance.html')"
          class="w-60"
        />
      </SettingRow>
    </SettingPanel>

    <SettingPanel :title="$gettext('Control Mode')">
      <SettingRow
        :title="$gettext('Nginx Control Mode')"
        :description="$gettext('How Nginx UI reaches the Nginx it manages.')"
        path="nginx.host_mode"
        stacked
      >
        <div v-if="!isEditingControl" class="flex flex-wrap items-center gap-2">
          <ATag v-if="currentMode === 'host_via_ssh'" color="orange">
            {{ $gettext('Host via SSH') }}
          </ATag>
          <ATag v-else-if="currentMode === 'external_container'" color="blue">
            {{ $gettext('External Docker Container') }}
          </ATag>
          <ATag v-else color="green">
            {{ $gettext('Local') }}
          </ATag>
          <span v-if="currentMode === 'external_container'">
            {{ data.nginx.container_name }}
          </span>
          <ATag v-if="currentMode === 'host_via_ssh' && data.nginx.host_access_mode === 'sftp'" color="blue">
            {{ $gettext('Compatibility (SFTP)') }}
          </ATag>
          <ATag v-else-if="currentMode === 'host_via_ssh' && data.nginx.host_access_mode === 'mounted'" color="green">
            {{ $gettext('High performance (mounted)') }}
          </ATag>
          <AButton size="small" @click="beginControlEdit">
            <EditOutlined />
            {{ $gettext('Edit') }}
          </AButton>
        </div>
        <AAlert
          v-if="!isEditingControl && currentMode === 'host_via_ssh' && data.nginx.host_access_mode === 'sftp'"
          type="info"
          show-icon
          class="mt-3"
          :title="$gettext('High-performance mode is available')"
          :description="$gettext('Compatibility mode works entirely over SSH. For lower file-access latency, configure bind mounts and switch to high-performance mode after recreating the container.')"
        >
          <template #action>
            <AButton size="small" @click="openSSHSetup">
              {{ $gettext('Review high-performance setup') }}
              <ArrowRightOutlined />
            </AButton>
          </template>
        </AAlert>
        <template v-if="isEditingControl">
          <ARadioGroup
            :value="selectedMode"
            @update:value="onModeChange"
          >
            <ARadio value="local">
              {{ $gettext('Local / Bundled') }}
            </ARadio>
            <ARadio value="external_container">
              {{ $gettext('External Container') }}
            </ARadio>
            <ARadio value="host_via_ssh">
              {{ $gettext('Host via SSH') }}
            </ARadio>
          </ARadioGroup>
        </template>
        <div v-if="isEditingControl && selectedMode === 'host_via_ssh'" class="mt-3">
          <AButton type="primary" @click="openSSHSetup">
            {{ $gettext('Open SSH setup wizard') }}
            <ArrowRightOutlined />
          </AButton>
        </div>
        <SettingRow
          v-if="isEditingControl && selectedMode === 'external_container'"
          :title="$gettext('External Docker Container')"
          :error="hasContainerNameError ? $gettext('This field is required') : undefined"
          class="mt-3"
        >
          <AInput
            v-model:value="containerName"
            placeholder="nginx"
            :status="hasContainerNameError ? 'error' : undefined"
            class="w-60"
          />
        </SettingRow>
        <div v-if="isEditingControl" class="mt-3 flex flex-wrap gap-2">
          <AButton
            v-if="selectedMode !== 'host_via_ssh'"
            type="primary"
            :loading="isSavingControl"
            @click="saveControlSettings"
          >
            <SaveOutlined />
            {{ $gettext('Save') }}
          </AButton>
          <AButton :disabled="isSavingControl" @click="cancelControlEdit">
            <CloseOutlined />
            {{ $gettext('Cancel') }}
          </AButton>
        </div>
      </SettingRow>
    </SettingPanel>

    <SettingPanel :title="$gettext('Paths and Commands')">
      <SettingRow
        :title="$gettext('Nginx Access Log Path')"
        path="nginx.access_log_path"
        config-file="nginx"
        :value="data.nginx.access_log_path"
      />
      <SettingRow
        :title="$gettext('Nginx Error Log Path')"
        path="nginx.error_log_path"
        config-file="nginx"
        :value="data.nginx.error_log_path"
      />
      <SettingRow
        :title="$gettext('Nginx Configurations Directory')"
        path="nginx.config_dir"
        config-file="nginx"
        :value="data.nginx.config_dir"
      />
      <SettingRow
        :title="$gettext('Nginx Configuration Path')"
        path="nginx.config_path"
        config-file="nginx"
        :value="data.nginx.config_path"
      />
      <SettingRow
        v-if="data.nginx.log_dir_white_list?.length"
        :title="$gettext('Nginx Log Directory Whitelist')"
        path="nginx.log_dir_white_list"
        config-file="nginx"
      >
        <div class="text-right text-gray-500">
          <div
            v-for="dir in data.nginx.log_dir_white_list"
            :key="dir"
          >
            {{ dir }}
          </div>
        </div>
      </SettingRow>
      <SettingRow
        v-else
        :title="$gettext('Nginx Log Directory Whitelist')"
        path="nginx.log_dir_white_list"
        config-file="nginx"
        :value="null"
      />
      <SettingRow
        :title="$gettext('Nginx PID Path')"
        path="nginx.pid_path"
        config-file="nginx"
        :value="data.nginx.pid_path"
      />
      <SettingRow
        :title="$gettext('Nginx Test Config Command')"
        path="nginx.test_config_cmd"
        config-file="nginx"
        :value="data.nginx.test_config_cmd"
      />
      <SettingRow
        :title="$gettext('Nginx Reload Command')"
        path="nginx.reload_cmd"
        config-file="nginx"
        :value="data.nginx.reload_cmd"
      />
      <SettingRow
        :title="$gettext('Nginx Restart Command')"
        path="nginx.restart_cmd"
        config-file="nginx"
        :value="data.nginx.restart_cmd"
      />
    </SettingPanel>
  </div>
</template>
