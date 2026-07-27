<script setup lang="ts">
import type { SetupParams } from '@/api/host_setup'
import { computed, ref, watch } from 'vue'

const params = defineModel<SetupParams>('params', { required: true })

const hostInput = ref(params.value.host_address ?? '')
watch(hostInput, v => {
  params.value.host_address = v
  params.value.use_host_gateway = v.startsWith('host.docker.internal')
})

watch(() => params.value.service_manager, manager => {
  if (manager === 'launchd') {
    Object.assign(params.value, {
      launchd_service: 'homebrew.mxcl.nginx',
      launchctl_path: '/bin/launchctl',
      nginx_sbin_path: '/opt/homebrew/bin/nginx',
      host_config_dir: '/opt/homebrew/etc/nginx',
      host_log_dir: '/opt/homebrew/var/log/nginx',
      pid_path: '/opt/homebrew/var/run/nginx.pid',
    })
    return
  }
  Object.assign(params.value, {
    systemd_unit: 'nginx.service',
    systemctl_path: '/bin/systemctl',
    nginx_sbin_path: '/usr/sbin/nginx',
    host_config_dir: '/etc/nginx',
    host_log_dir: '/var/log/nginx',
    pid_path: '/var/run/nginx.pid',
  })
})

const remoteWarning = computed<boolean>(() => {
  const host = (hostInput.value.split(':')[0] || '').trim()
  if (!host)
    return false
  if (host === 'host.docker.internal')
    return false
  if (host === 'localhost' || host === '::1')
    return false
  if (/^127\./.test(host))
    return false
  if (/^172\.(?:1[6-9]|2\d|3[01])\.0\.1$/.test(host))
    return false
  return true
})
</script>

<template>
  <div class="space-y-4">
    <AFormItem :label="$gettext('Host address (host:port)')" required>
      <AInput v-model:value="hostInput" placeholder="host.docker.internal:22" />
    </AFormItem>

    <AFormItem :label="$gettext('SSH user')" required>
      <AInput v-model:value="params.host_user" placeholder="nginxui" />
    </AFormItem>

    <AFormItem :label="$gettext('Host platform')" required>
      <ASegmented
        v-model:value="params.service_manager"
        :options="[
          { label: $gettext('Linux (systemd)'), value: 'systemd' },
          { label: $gettext('macOS (Homebrew)'), value: 'launchd' },
        ]"
      />
    </AFormItem>

    <AFormItem v-if="params.service_manager === 'systemd'" :label="$gettext('systemd unit')">
      <AInput v-model:value="params.systemd_unit" placeholder="nginx.service" />
    </AFormItem>

    <AFormItem v-if="params.service_manager === 'systemd'" :label="$gettext('systemctl path')">
      <AInput v-model:value="params.systemctl_path" placeholder="/bin/systemctl" />
    </AFormItem>

    <AFormItem v-if="params.service_manager === 'launchd'" :label="$gettext('launchd service label')">
      <AInput v-model:value="params.launchd_service" placeholder="homebrew.mxcl.nginx" />
    </AFormItem>

    <AFormItem v-if="params.service_manager === 'launchd'" :label="$gettext('launchctl path')">
      <AInput v-model:value="params.launchctl_path" placeholder="/bin/launchctl" />
    </AFormItem>

    <AFormItem :label="$gettext('nginx executable path')" required>
      <AInput v-model:value="params.nginx_sbin_path" />
    </AFormItem>

    <AFormItem :label="$gettext('nginx config directory')" required>
      <AInput v-model:value="params.host_config_dir" />
    </AFormItem>

    <AFormItem :label="$gettext('nginx log directory')" required>
      <AInput v-model:value="params.host_log_dir" />
    </AFormItem>

    <AFormItem :label="$gettext('nginx PID file')" required>
      <AInput v-model:value="params.pid_path" />
    </AFormItem>

    <AAlert
      v-if="remoteWarning"
      type="warning"
      show-icon
    >
      <template #message>
        {{ $gettext('Remote address detected') }}
      </template>
      <template #description>
        {{ $gettext('This mode only supports nginx-ui and target nginx on the same host.') }}
        {{ $gettext('bind-mount cannot reach remote filesystems — config editing, log viewing and certificate management will fail.') }}
        <a
          href="/docs/guide/cluster-node-cross-host.html"
          target="_blank"
          rel="noopener"
        >
          {{ $gettext('Use the cluster Node guide for cross-host setups.') }}
        </a>
      </template>
    </AAlert>
  </div>
</template>
