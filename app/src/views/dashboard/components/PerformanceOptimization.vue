<script setup lang="ts">
import type { NginxConfigInfo, NginxPerfOpt } from '@/api/ngx'
import type { CheckedType } from '@/types'
import ngx from '@/api/ngx'
import {
  SettingOutlined,
} from '@ant-design/icons-vue'
import { message } from 'ant-design-vue'

// Size units
const sizeUnits = ['k', 'm', 'g']

// Size values and units
const maxBodySizeValue = ref<number>(1)
const maxBodySizeUnit = ref<string>('m')
const headerBufferSizeValue = ref<number>(1)
const headerBufferSizeUnit = ref<string>('k')
const bodyBufferSizeValue = ref<number>(8)
const bodyBufferSizeUnit = ref<string>('k')

// Performance settings modal
const visible = ref(false)
const loading = ref(false)
const performanceConfig = ref<NginxConfigInfo>({
  worker_processes: 1,
  worker_connections: 1024,
  process_mode: 'manual',
  keepalive_timeout: 65,
  gzip: 'off',
  gzip_min_length: 1,
  gzip_comp_level: 1,
  client_max_body_size: '1m',
  server_names_hash_bucket_size: 32,
  client_header_buffer_size: '1k',
  client_body_buffer_size: '8k',
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
    const data = await ngx.get_performance()
    performanceConfig.value = data

    // Parse size values and units
    parseSizeValues()
  }
  catch (error) {
    console.error('Failed to get Nginx performance settings:', error)
    message.error($gettext('Failed to get Nginx performance settings'))
  }
  finally {
    loading.value = false
  }
}

// Parse size values from config
function parseSizeValues() {
  // Parse client_max_body_size
  const maxBodySize = performanceConfig.value.client_max_body_size
  const maxBodyMatch = maxBodySize.match(/^(\d+)([kmg])?$/i)
  if (maxBodyMatch) {
    maxBodySizeValue.value = Number.parseInt(maxBodyMatch[1])
    maxBodySizeUnit.value = (maxBodyMatch[2] || 'm').toLowerCase()
  }

  // Parse client_header_buffer_size
  const headerSize = performanceConfig.value.client_header_buffer_size
  const headerMatch = headerSize.match(/^(\d+)([kmg])?$/i)
  if (headerMatch) {
    headerBufferSizeValue.value = Number.parseInt(headerMatch[1])
    headerBufferSizeUnit.value = (headerMatch[2] || 'k').toLowerCase()
  }

  // Parse client_body_buffer_size
  const bodySize = performanceConfig.value.client_body_buffer_size
  const bodyMatch = bodySize.match(/^(\d+)([kmg])?$/i)
  if (bodyMatch) {
    bodyBufferSizeValue.value = Number.parseInt(bodyMatch[1])
    bodyBufferSizeUnit.value = (bodyMatch[2] || 'k').toLowerCase()
  }
}

// Format size values before saving
function formatSizeValues() {
  performanceConfig.value.client_max_body_size = `${maxBodySizeValue.value}${maxBodySizeUnit.value}`
  performanceConfig.value.client_header_buffer_size = `${headerBufferSizeValue.value}${headerBufferSizeUnit.value}`
  performanceConfig.value.client_body_buffer_size = `${bodyBufferSizeValue.value}${bodyBufferSizeUnit.value}`
}

// Save performance settings
async function savePerformanceSettings() {
  loading.value = true
  try {
    // Format size values
    formatSizeValues()

    const params: NginxPerfOpt = {
      worker_processes: performanceConfig.value.process_mode === 'auto' ? 'auto' : performanceConfig.value.worker_processes.toString(),
      worker_connections: performanceConfig.value.worker_connections.toString(),
      keepalive_timeout: performanceConfig.value.keepalive_timeout.toString(),
      gzip: performanceConfig.value.gzip,
      gzip_min_length: performanceConfig.value.gzip_min_length.toString(),
      gzip_comp_level: performanceConfig.value.gzip_comp_level.toString(),
      client_max_body_size: performanceConfig.value.client_max_body_size,
      server_names_hash_bucket_size: performanceConfig.value.server_names_hash_bucket_size.toString(),
      client_header_buffer_size: performanceConfig.value.client_header_buffer_size,
      client_body_buffer_size: performanceConfig.value.client_body_buffer_size,
    }
    const data = await ngx.update_performance(params)
    performanceConfig.value = data

    // Parse the returned values
    parseSizeValues()

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

// Toggle worker process mode
function handleProcessModeChange(checked: CheckedType) {
  performanceConfig.value.process_mode = checked ? 'auto' : 'manual'
  if (checked) {
    performanceConfig.value.worker_processes = navigator.hardwareConcurrency || 4
  }
}

// Toggle GZIP compression
function handleGzipChange(checked: CheckedType) {
  performanceConfig.value.gzip = checked ? 'on' : 'off'
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
      {{ $gettext('Optimize Performance') }}
    </AButton>

    <!-- Performance Optimization Modal -->
    <AModal
      v-model:open="visible"
      :title="$gettext('Optimize Nginx Performance')"
      :mask-closable="false"
      :ok-button-props="{ loading }"
      @ok="savePerformanceSettings"
    >
      <ASpin :spinning="loading">
        <AForm layout="vertical">
          <AFormItem
            :label="$gettext('Worker Processes')"
            :help="$gettext('Number of concurrent worker processes, auto sets to CPU core count')"
          >
            <ASpace>
              <ASwitch
                :checked="performanceConfig.process_mode === 'auto'"
                :checked-children="$gettext('Auto')"
                :un-checked-children="$gettext('Manual')"
                @change="handleProcessModeChange"
              />
              <AInputNumber
                v-if="performanceConfig.process_mode !== 'auto'"
                v-model:value="performanceConfig.worker_processes"
                :min="1"
                :max="32"
                style="width: 120px"
              />
              <span v-else>{{ performanceConfig.worker_processes }} ({{ $gettext('Auto') }})</span>
            </ASpace>
          </AFormItem>

          <AFormItem
            :label="$gettext('Worker Connections')"
            :help="$gettext('Maximum number of concurrent connections')"
          >
            <AInputNumber
              v-model:value="performanceConfig.worker_connections"
              :min="512"
              :max="65536"
              style="width: 120px"
            />
          </AFormItem>

          <AFormItem
            :label="$gettext('Keepalive Timeout')"
            :help="$gettext('Connection timeout period')"
          >
            <ASpace>
              <AInputNumber
                v-model:value="performanceConfig.keepalive_timeout"
                :min="0"
                :max="999"
                style="width: 120px"
              />
              <span>{{ $gettext('seconds') }}</span>
            </ASpace>
          </AFormItem>

          <AFormItem
            :label="$gettext('GZIP Compression')"
            :help="$gettext('Enable compression for content transfer')"
          >
            <ASwitch
              :checked="performanceConfig.gzip === 'on'"
              :checked-children="$gettext('On')"
              :un-checked-children="$gettext('Off')"
              @change="handleGzipChange"
            />
          </AFormItem>

          <AFormItem
            :label="$gettext('GZIP Min Length')"
            :help="$gettext('Minimum file size for compression')"
          >
            <ASpace>
              <AInputNumber
                v-model:value="performanceConfig.gzip_min_length"
                :min="0"
                style="width: 120px"
              />
              <span>{{ $gettext('KB') }}</span>
            </ASpace>
          </AFormItem>

          <AFormItem
            :label="$gettext('GZIP Compression Level')"
            :help="$gettext('Compression level, 1 is lowest, 9 is highest')"
          >
            <AInputNumber
              v-model:value="performanceConfig.gzip_comp_level"
              :min="1"
              :max="9"
              style="width: 120px"
            />
          </AFormItem>

          <AFormItem
            :label="$gettext('Client Max Body Size')"
            :help="$gettext('Maximum client request body size')"
          >
            <AInputGroup compact style="width: 180px">
              <AInputNumber
                v-model:value="maxBodySizeValue"
                :min="1"
                style="width: 120px"
              />
              <ASelect v-model:value="maxBodySizeUnit" style="width: 60px">
                <ASelectOption v-for="unit in sizeUnits" :key="unit" :value="unit">
                  {{ unit.toUpperCase() }}
                </ASelectOption>
              </ASelect>
            </AInputGroup>
          </AFormItem>

          <AFormItem
            :label="$gettext('Server Names Hash Bucket Size')"
            :help="$gettext('Server names hash table size')"
          >
            <AInputNumber
              v-model:value="performanceConfig.server_names_hash_bucket_size"
              :min="32"
              :step="32"
              style="width: 120px"
            />
          </AFormItem>

          <AFormItem
            :label="$gettext('Client Header Buffer Size')"
            :help="$gettext('Client request header buffer size')"
          >
            <AInputGroup compact style="width: 180px">
              <AInputNumber
                v-model:value="headerBufferSizeValue"
                :min="1"
                style="width: 120px"
              />
              <ASelect v-model:value="headerBufferSizeUnit" style="width: 60px">
                <ASelectOption v-for="unit in sizeUnits" :key="unit" :value="unit">
                  {{ unit.toUpperCase() }}
                </ASelectOption>
              </ASelect>
            </AInputGroup>
          </AFormItem>

          <AFormItem
            :label="$gettext('Client Body Buffer Size')"
            :help="$gettext('Client request body buffer size')"
          >
            <AInputGroup compact style="width: 180px">
              <AInputNumber
                v-model:value="bodyBufferSizeValue"
                :min="1"
                style="width: 120px"
              />
              <ASelect v-model:value="bodyBufferSizeUnit" style="width: 60px">
                <ASelectOption v-for="unit in sizeUnits" :key="unit" :value="unit">
                  {{ unit.toUpperCase() }}
                </ASelectOption>
              </ASelect>
            </AInputGroup>
          </AFormItem>
        </AForm>
      </ASpin>
    </AModal>
  </div>
</template>
