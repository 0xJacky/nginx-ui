<script setup lang="ts">
import { computed, ref } from 'vue'
import NginxHostSetupWizard from '../components/NginxHostSetup/Wizard.vue'
import useSystemSettingsStore from '../store'

const systemSettingsStore = useSystemSettingsStore()
const { data } = storeToRefs(systemSettingsStore)

const showWizard = ref(false)

const modeSelection = computed(() => {
  if (data.value.nginx.host_mode === 'ssh')
    return 'ssh'
  if (data.value.nginx.container_name)
    return 'external_container'
  return 'local'
})

function onModeChange(value: string) {
  if (value === 'ssh') {
    data.value.nginx.host_mode = 'ssh'
  }
  else {
    data.value.nginx.host_mode = ''
    if (value === 'local') {
      // user selected local — leave container_name as-is; emptying it would lose history
      // settings save endpoint validates the final combination
    }
  }
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
        {{ $gettext('Mounted directory') }}: /etc/nginx/maintenance
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
      <ARadioGroup
        :value="modeSelection"
        @update:value="onModeChange"
      >
        <ARadio value="local">
          {{ $gettext('Local / Bundled') }}
        </ARadio>
        <ARadio value="external_container">
          {{ $gettext('External Container') }}
        </ARadio>
        <ARadio value="ssh">
          {{ $gettext('Host via SSH') }}
        </ARadio>
      </ARadioGroup>
      <div v-if="data.nginx.host_mode === 'ssh'" class="mt-3">
        <AButton @click="showWizard = !showWizard">
          {{ showWizard ? $gettext('Hide setup wizard') : $gettext('Open setup wizard') }}
        </AButton>
      </div>
      <div v-else-if="data.nginx.container_name" class="mt-3">
        <ATag color="blue">
          {{ $gettext('External Docker Container') }}
        </ATag>
        {{ data.nginx.container_name }}
      </div>
      <div v-else class="mt-3">
        <ATag color="green">
          {{ $gettext('Local') }}
        </ATag>
      </div>
    </AFormItem>

    <NginxHostSetupWizard v-if="showWizard" class="mt-4" />
  </AForm>
</template>

<style lang="less" scoped>

</style>
