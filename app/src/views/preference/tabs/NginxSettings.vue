<script setup lang="ts">
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
        :placeholder="$gettext('maintenance.html')" />
      <div class="text-secondary mt-1">
        {{$gettext('Mounted directory')}}: /etc/nginx/maintenance
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
      <div v-if="data.nginx.container_name">
        <ATag color="blue" tag>
          {{ $gettext('External Docker Container') }}
        </ATag>
        {{ data.nginx.container_name }}
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

</style>
