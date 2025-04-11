<script setup lang="ts">
import type { NginxPerformanceInfo } from '@/api/ngx'
import {
  ApiOutlined,
  CloudServerOutlined,
  DashboardOutlined,
  InfoCircleOutlined,
  ThunderboltOutlined,
} from '@ant-design/icons-vue'

const props = defineProps<{
  nginxInfo: NginxPerformanceInfo
}>()

// Calculate connection efficiency - requests per connection
const requestsPerConnection = computed(() => {
  if (props.nginxInfo.handled === 0) {
    return '0'
  }
  return (props.nginxInfo.requests / props.nginxInfo.handled).toFixed(2)
})

// Estimate maximum requests per second
const maxRPS = computed(() => {
  return props.nginxInfo.worker_processes * props.nginxInfo.worker_connections
})
</script>

<template>
  <div>
    <ARow :gutter="[16, 24]">
      <!-- Maximum RPS -->
      <ACol :xs="24" :sm="12" :md="8" :lg="6">
        <AStatistic
          :value="maxRPS"
          :value-style="{ color: '#1890ff', fontSize: '24px' }"
        >
          <template #prefix>
            <ThunderboltOutlined />
          </template>
          <template #title>
            {{ $gettext('Max Requests Per Second') }}
            <ATooltip :title="$gettext('Calculated based on worker_processes * worker_connections. Actual performance depends on hardware, configuration, and workload')">
              <InfoCircleOutlined class="ml-1 text-gray-500" />
            </ATooltip>
          </template>
        </AStatistic>
        <div class="text-xs text-gray-500 mt-1">
          worker_processes ({{ nginxInfo.worker_processes }}) × worker_connections ({{ nginxInfo.worker_connections }})
        </div>
      </ACol>

      <!-- Maximum concurrent connections -->
      <ACol :xs="24" :sm="12" :md="8" :lg="6">
        <AStatistic
          :title="$gettext('Max Concurrent Connections')"
          :value="nginxInfo.worker_processes * nginxInfo.worker_connections"
          :value-style="{ color: '#52c41a', fontSize: '24px' }"
        >
          <template #prefix>
            <ApiOutlined />
          </template>
        </AStatistic>
        <div class="text-xs text-gray-500 mt-1">
          {{ $gettext('Current usage') }}: {{ ((nginxInfo.active / (nginxInfo.worker_processes * nginxInfo.worker_connections)) * 100).toFixed(2) }}%
        </div>
      </ACol>

      <!-- Requests per connection -->
      <ACol :xs="24" :sm="12" :md="8" :lg="6">
        <AStatistic
          :value="requestsPerConnection"
          :precision="2"
          :value-style="{ color: '#3a7f99', fontSize: '24px' }"
        >
          <template #title>
            {{ $gettext('Requests Per Connection') }}
            <ATooltip :title="$gettext('Total Requests / Total Connections')">
              <InfoCircleOutlined class="ml-1 text-gray-500" />
            </ATooltip>
          </template>
          <template #prefix>
            <DashboardOutlined />
          </template>
        </AStatistic>
        <div class="text-xs text-gray-500 mt-1">
          {{ $gettext('Higher value means better connection reuse') }}
        </div>
      </ACol>

      <!-- Total Nginx processes -->
      <ACol :xs="24" :sm="12" :md="8" :lg="6">
        <AStatistic
          :title="$gettext('Total Nginx Processes')"
          :value="nginxInfo.workers + nginxInfo.master + nginxInfo.cache + nginxInfo.other"
          :value-style="{ color: '#722ed1', fontSize: '24px' }"
        >
          <template #prefix>
            <CloudServerOutlined />
          </template>
        </AStatistic>
        <div class="text-xs text-gray-500 mt-1">
          {{ $gettext('Workers') }}: {{ nginxInfo.workers }}, {{ $gettext('Master') }}: {{ nginxInfo.master }}, {{ $gettext('Others') }}: {{ nginxInfo.cache + nginxInfo.other }}
        </div>
      </ACol>
    </ARow>
  </div>
</template>
