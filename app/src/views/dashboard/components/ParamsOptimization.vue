<script setup lang="ts">
import type { NginxConfigInfo, NginxPerfOpt } from '@/api/ngx'
import {
  SettingOutlined,
} from '@ant-design/icons-vue'
import { message } from 'ant-design-vue'
import ngx from '@/api/ngx'
import PerformanceConfig from './ParamsOpt/PerformanceConfig.vue'
import ProxyCacheConfig from './ParamsOpt/ProxyCacheConfig.vue'

// Performance settings modal
const visible = ref(false)
const loading = ref(false)
const data = ref<NginxConfigInfo>({
  worker_processes: '4',
  worker_connections: 1024,
  process_mode: 'manual',
  keepalive_timeout: '65s',
  gzip: 'off',
  gzip_min_length: 1,
  gzip_comp_level: 1,
  client_max_body_size: '1m',
  server_names_hash_bucket_size: '32',
  client_header_buffer_size: '1k',
  client_body_buffer_size: '8k',
  proxy_cache: {
    enabled: false,
    path: '/var/cache/nginx/proxy_cache',
    levels: '1:2',
    use_temp_path: 'off',
    keys_zone: 'proxy_cache:10m',
    inactive: '60m',
    max_size: '1g',
    min_free: '',
    manager_files: '',
    manager_sleep: '',
    manager_threshold: '',
    loader_files: '',
    loader_sleep: '',
    loader_threshold: '',
    purger: 'off',
    purger_files: '',
    purger_sleep: '',
    purger_threshold: '',
  },
})

// Open modal and load performance settings
async function openPerformanceModal() {
  visible.value = true
  await fetchPerformanceSettings()
}

// Load performance settings
async function fetchPerformanceSettings() {
  loading.value = true
  try {
    data.value = await ngx.get_performance()
  }
  catch (error) {
    console.error('Failed to get Nginx performance settings:', error)
    message.error($gettext('Failed to get Nginx performance settings'))
  }
  finally {
    loading.value = false
  }
}

// Save performance settings
async function savePerformanceSettings() {
  loading.value = true
  try {
    const params: NginxPerfOpt = {
      worker_processes: data.value.process_mode === 'auto' ? 'auto' : data.value.worker_processes.toString(),
      worker_connections: data.value.worker_connections.toString(),
      keepalive_timeout: data.value.keepalive_timeout.toString(),
      gzip: data.value.gzip,
      gzip_min_length: data.value.gzip_min_length.toString(),
      gzip_comp_level: data.value.gzip_comp_level.toString(),
      client_max_body_size: data.value.client_max_body_size,
      server_names_hash_bucket_size: data.value.server_names_hash_bucket_size.toString(),
      client_header_buffer_size: data.value.client_header_buffer_size,
      client_body_buffer_size: data.value.client_body_buffer_size,
      proxy_cache: data.value.proxy_cache,
    }
    await ngx.update_performance(params)
    message.success($gettext('Performance settings saved successfully'))
  }
  catch (error) {
    console.error('Failed to save Nginx performance settings:', error)
    message.error($gettext('Failed to save Nginx performance settings'))
  }
  finally {
    loading.value = false
  }
}
</script>

<template>
  <div>
    <!-- Performance Optimization Button -->
    <AButton
      type="link"
      size="small"
      @click="openPerformanceModal"
    >
      <template #icon>
        <SettingOutlined />
      </template>
      {{ $gettext('Params Optimization') }}
    </AButton>

    <!-- Performance Optimization Modal -->
    <AModal
      v-model:open="visible"
      :title="$gettext('Params Optimization')"
      :mask-closable="false"
      :ok-button-props="{ loading }"
      @ok="savePerformanceSettings"
    >
      <ATabs>
        <ATabPane key="performance" :tab="$gettext('Performance')">
          <PerformanceConfig v-model="data" />
        </ATabPane>
        <ATabPane key="cache" :tab="$gettext('Cache')">
          <ProxyCacheConfig v-model="data.proxy_cache" />
        </ATabPane>
      </ATabs>
    </AModal>
  </div>
</template>
