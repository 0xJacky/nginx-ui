<script setup lang="ts">
import SensitiveString from '@/components/SensitiveString'
import useSystemSettingsStore from '../store'

const systemSettingsStore = useSystemSettingsStore()
const { data } = storeToRefs(systemSettingsStore)
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
      <SensitiveString path="nginx.access_log_path" :value="data.nginx.access_log_path" />
    </AFormItem>
    <AFormItem :label="$gettext('Nginx Error Log Path')">
      <SensitiveString path="nginx.error_log_path" :value="data.nginx.error_log_path" />
    </AFormItem>
    <AFormItem :label="$gettext('Nginx Configurations Directory')">
      <SensitiveString path="nginx.config_dir" :value="data.nginx.config_dir" />
    </AFormItem>
    <AFormItem :label="$gettext('Nginx Configuration Path')">
      <SensitiveString path="nginx.config_path" :value="data.nginx.config_path" />
    </AFormItem>
    <AFormItem :label="$gettext('Nginx Log Directory Whitelist')">
      <SensitiveString
        v-if="data.nginx.log_dir_white_list?.length"
        path="nginx.log_dir_white_list"
        :value="data.nginx.log_dir_white_list"
      />
    </AFormItem>
    <AFormItem :label="$gettext('Nginx PID Path')">
      <SensitiveString path="nginx.pid_path" :value="data.nginx.pid_path" />
    </AFormItem>
    <AFormItem :label="$gettext('Nginx Test Config Command')">
      <SensitiveString path="nginx.test_config_cmd" :value="data.nginx.test_config_cmd" />
    </AFormItem>
    <AFormItem :label="$gettext('Nginx Reload Command')">
      <SensitiveString path="nginx.reload_cmd" :value="data.nginx.reload_cmd" />
    </AFormItem>
    <AFormItem :label="$gettext('Nginx Restart Command')">
      <SensitiveString path="nginx.restart_cmd" :value="data.nginx.restart_cmd" />
    </AFormItem>
    <AFormItem :label="$gettext('Nginx Control Mode')">
      <div v-if="data.nginx.container_name" class="control-mode">
        <ATag color="blue" tag>
          {{ $gettext('External Docker Container') }}
        </ATag>
        <SensitiveString path="nginx.container_name" :value="data.nginx.container_name" />
      </div>
      <div v-else>
        <ATag color="green" tag>
          {{ $gettext('Local') }}
        </ATag>
      </div>
    </AFormItem>
  </AForm>
</template>

<style lang="less" scoped>
.control-mode {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
</style>
