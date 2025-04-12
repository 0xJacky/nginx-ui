<script setup lang="ts">
import type { NginxConfigInfo } from '@/api/ngx'
import type { CheckedType } from '@/types'
import SizeInput from './SizeInput.vue'
import TimeInput from './TimeInput.vue'

const performanceConfig = defineModel<NginxConfigInfo>({
  default: reactive({
    proxy_cache: {},
  }),
})

const workerProcessAutoMode = ref<boolean>(true)

function handleWorkerProcessAutoModeChange(checked: CheckedType) {
  if (checked) {
    performanceConfig.value.worker_processes = 'auto'
  }
}
</script>

<template>
  <AForm layout="vertical">
    <AFormItem
      :label="$gettext('Worker Processes')"
      :help="$gettext('Number of concurrent worker processes, auto sets to CPU core count')"
    >
      <ASpace>
        <ASwitch
          v-model:checked="workerProcessAutoMode"
          :checked-children="$gettext('Auto')"
          :un-checked-children="$gettext('Manual')"
          @change="handleWorkerProcessAutoModeChange"
        />
        <AInputNumber
          v-if="!workerProcessAutoMode"
          v-model:value="performanceConfig.worker_processes"
          :min="1"
          :max="32"
          style="width: 120px"
          string-mode
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
      <TimeInput v-model="performanceConfig.keepalive_timeout" />
    </AFormItem>

    <AFormItem
      :label="$gettext('GZIP Compression')"
      :help="$gettext('Enable compression for content transfer')"
    >
      <ASwitch
        v-model:checked="performanceConfig.gzip"
        :checked-children="$gettext('On')"
        :un-checked-children="$gettext('Off')"
        checked-value="on"
        un-checked-value="off"
      />
    </AFormItem>

    <AFormItem
      :label="$gettext('GZIP Min Length')"
      :help="$gettext('Minimum file size for compression')"
    >
      <AInputNumber v-model:value="performanceConfig.gzip_min_length" />
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
      <SizeInput v-model="performanceConfig.client_max_body_size" />
    </AFormItem>

    <AFormItem
      :label="$gettext('Server Names Hash Bucket Size')"
      :help="$gettext('Server names hash table size')"
    >
      <SizeInput v-model="performanceConfig.server_names_hash_bucket_size" />
    </AFormItem>

    <AFormItem
      :label="$gettext('Client Header Buffer Size')"
      :help="$gettext('Client request header buffer size')"
    >
      <SizeInput v-model="performanceConfig.client_header_buffer_size" />
    </AFormItem>
    <!-- Client Body Buffer Size -->
    <AFormItem
      :label="$gettext('Client Body Buffer Size')"
      :help="$gettext('Client request body buffer size')"
    >
      <SizeInput v-model="performanceConfig.client_body_buffer_size" />
    </AFormItem>
  </AForm>
</template>
