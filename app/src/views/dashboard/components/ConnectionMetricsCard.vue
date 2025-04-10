<script setup lang="ts">
import type { NginxPerformanceInfo } from '@/api/ngx'
import { InfoCircleOutlined } from '@ant-design/icons-vue'
import { computed, defineProps } from 'vue'

const props = defineProps<{
  nginxInfo: NginxPerformanceInfo
}>()

// Active connections percentage
const activeConnectionsPercent = computed(() => {
  const maxConnections = props.nginxInfo.worker_connections * props.nginxInfo.worker_processes
  return Number(((props.nginxInfo.active / maxConnections) * 100).toFixed(2))
})

// Worker processes usage percentage
const workerProcessesPercent = computed(() => {
  return Number(((props.nginxInfo.workers / props.nginxInfo.worker_processes) * 100).toFixed(2))
})

// Requests per connection
const requestsPerConnection = computed(() => {
  if (props.nginxInfo.handled === 0) {
    return '0'
  }
  return (props.nginxInfo.requests / props.nginxInfo.handled).toFixed(2)
})

// Format numbers
function formatNumber(num: number): string {
  if (num >= 1000000) {
    return `${(num / 1000000).toFixed(2)}M`
  }
  else if (num >= 1000) {
    return `${(num / 1000).toFixed(2)}K`
  }
  return num.toString()
}
</script>

<template>
  <ARow :gutter="[16, 16]">
    <!-- Current active connections -->
    <ACol :xs="24" :sm="12" :md="12" :lg="6">
      <ACard class="h-full" :bordered="false" :body-style="{ padding: '20px', height: '100%' }">
        <div class="flex flex-col h-full">
          <div class="mb-2 text-gray-500 font-medium truncate">
            {{ $gettext('Current active connections') }}
          </div>
          <div class="flex items-baseline mb-2">
            <span class="text-2xl font-bold mr-2">{{ nginxInfo.active }}</span>
            <span class="text-gray-500 text-sm">/ {{ nginxInfo.worker_connections * nginxInfo.worker_processes }}</span>
          </div>
          <AProgress
            :percent="activeConnectionsPercent"
            :format="percent => `${percent?.toFixed(2)}%`"
            :status="activeConnectionsPercent > 80 ? 'exception' : 'normal'"
            size="small"
          />
        </div>
      </ACard>
    </ACol>

    <!-- Worker processes -->
    <ACol :xs="24" :sm="12" :md="12" :lg="6">
      <ACard class="h-full" :bordered="false" :body-style="{ padding: '20px', height: '100%' }">
        <div class="flex flex-col h-full">
          <div class="mb-2 text-gray-500 font-medium truncate">
            {{ $gettext('Worker Processes') }}
          </div>
          <div class="flex items-baseline mb-2">
            <span class="text-2xl font-bold mr-2">{{ nginxInfo.workers }}</span>
            <span class="text-gray-500 text-sm">/ {{ nginxInfo.worker_processes }}</span>
          </div>
          <AProgress
            :percent="workerProcessesPercent"
            size="small"
            status="active"
          />
          <div class="mt-2 text-xs text-gray-500 overflow-hidden text-ellipsis">
            {{ $gettext('Total Nginx processes') }}: {{ nginxInfo.workers + nginxInfo.master + nginxInfo.cache + nginxInfo.other }}
            <Tooltip :title="$gettext('Includes master process, worker processes, cache processes, and other Nginx processes')">
              <InfoCircleOutlined class="ml-1" />
            </Tooltip>
          </div>
        </div>
      </ACard>
    </ACol>

    <!-- Requests per connection -->
    <ACol :xs="24" :sm="12" :md="12" :lg="6">
      <ACard class="h-full" :bordered="false" :body-style="{ padding: '20px', height: '100%' }">
        <div class="flex flex-col h-full justify-between">
          <div>
            <div class="mb-2 text-gray-500 font-medium truncate">
              {{ $gettext('Requests per connection') }}
            </div>
            <div class="flex items-baseline mb-2">
              <span class="text-2xl font-bold">{{ requestsPerConnection }}</span>
              <Tooltip :title="$gettext('The average number of requests per connection, the higher the value, the higher the connection reuse efficiency')">
                <InfoCircleOutlined class="ml-2 text-gray-500" />
              </Tooltip>
            </div>
          </div>
          <div>
            <div class="text-xs text-gray-500 mb-1 truncate">
              {{ $gettext('Total requests') }}: {{ formatNumber(nginxInfo.requests) }}
            </div>
            <div class="text-xs text-gray-500 truncate">
              {{ $gettext('Total connections') }}: {{ formatNumber(nginxInfo.handled) }}
            </div>
          </div>
        </div>
      </ACard>
    </ACol>

    <!-- Resource utilization -->
    <ACol :xs="24" :sm="12" :md="12" :lg="6">
      <ACard class="h-full" :bordered="false" :body-style="{ padding: '20px', height: '100%' }">
        <div class="flex flex-col h-full justify-between">
          <div class="mb-2 text-gray-500 font-medium truncate">
            {{ $gettext('Resource Utilization') }}
          </div>
          <div class="flex items-center justify-center flex-grow">
            <AProgress
              type="dashboard"
              :percent="Math.round((Math.min(nginxInfo.cpu_usage / 100, 1) * 0.5 + Math.min(nginxInfo.active / (nginxInfo.worker_connections * nginxInfo.worker_processes), 1) * 0.5) * 100)"
              :width="80"
              status="active"
            />
          </div>
          <div class="mt-2 text-xs text-gray-500 text-center overflow-hidden text-ellipsis">
            {{ $gettext('Based on CPU usage and connection usage') }}
          </div>
        </div>
      </ACard>
    </ACol>
  </ARow>
</template>
