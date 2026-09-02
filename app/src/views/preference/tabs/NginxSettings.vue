<script setup lang="ts">
import type { NginxControlMode, NginxSettings } from '@/api/settings'
import { ArrowRightOutlined, CloseOutlined, EditOutlined, SaveOutlined } from '@ant-design/icons-vue'
import settingsApi from '@/api/settings'
import { TwoFACancelledError, use2FAModal } from '@/components/TwoFA'
import {
  applyNginxControlSettings,
  buildNginxControlPayload,
  cloneNginxSettings,
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
  <AForm layout="vertical">
    <AFormItem :label="$gettext('Stub Status Port')">
      <AInputNumber v-model:value="data.nginx.stub_status_port" />
    </AFormItem>
    <AFormItem :label="$gettext('Maintenance template (filename only)')">
      <AInput
        v-model:value="data.nginx.maintenance_template"
        :placeholder="$gettext('maintenance.html')"
      />
      <div class="text-secondary mt-1">
        {{ $gettext('Mounted directory') }}: {{ data.nginx.maintenance_dir }}
      </div>
      <div class="text-secondary mt-1">
        {{ $gettext('The file named <site name>.<filename> is used first; if it does not exist, the generic <filename> is used; if neither exists, the built-in Nginx UI maintenance page is used.') }}
      </div>
    </AFormItem>
    <AFormItem :label="$gettext('Nginx Access Log Path')">
      {{ data.nginx.access_log_path }}
    </AFormItem>
    <AFormItem :label="$gettext('Nginx Error Log Path')">
      {{ data.nginx.error_log_path }}
    </AFormItem>
    <AFormItem :label="$gettext('Nginx Configurations Directory')">
      {{ data.nginx.config_dir }}
    </AFormItem>
    <AFormItem :label="$gettext('Nginx Configuration Path')">
      <p>{{ data.nginx.config_path }}</p>
    </AFormItem>
    <AFormItem :label="$gettext('Nginx Log Directory Whitelist')">
      <div
        v-for="dir in data.nginx.log_dir_white_list"
        :key="dir"
        class="mb-2"
      >
        {{ dir }}
      </div>
    </AFormItem>
    <AFormItem :label="$gettext('Nginx PID Path')">
      {{ data.nginx.pid_path }}
    </AFormItem>
    <AFormItem :label="$gettext('Nginx Test Config Command')">
      <p>{{ data.nginx.test_config_cmd }}</p>
    </AFormItem>
    <AFormItem :label="$gettext('Nginx Reload Command')">
      {{ data.nginx.reload_cmd }}
    </AFormItem>
    <AFormItem :label="$gettext('Nginx Restart Command')">
      {{ data.nginx.restart_cmd }}
    </AFormItem>
    <AFormItem :label="$gettext('Nginx Control Mode')">
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
        :message="$gettext('High-performance mode is available')"
        :description="$gettext('Compatibility mode works entirely over SSH. For lower file-access latency, configure bind mounts and switch to high-performance mode after recreating the container.')"
      >
        <template #action>
          <AButton size="small" @click="openSSHSetup">
            {{ $gettext('Review high-performance setup') }}
            <ArrowRightOutlined />
          </AButton>
        </template>
      </AAlert>
      <template v-else>
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
    </AFormItem>

    <AFormItem
      v-if="isEditingControl && selectedMode === 'external_container'"
      :label="$gettext('External Docker Container')"
      :validate-status="hasContainerNameError ? 'error' : undefined"
      :help="hasContainerNameError ? $gettext('This field is required') : undefined"
    >
      <AInput v-model:value="containerName" placeholder="nginx" />
    </AFormItem>

    <div v-if="isEditingControl" class="mb-6 flex flex-wrap gap-2">
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
  </AForm>
</template>

<style lang="less" scoped>

</style>
