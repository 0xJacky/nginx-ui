<script setup lang="ts">
import type { NginxPerformanceInfo } from '@/api/ngx'

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
</script>

<template>
  <ARow :gutter="[16, 16]" class="h-full">
    <!-- Current active connections -->
    <ACol :xs="24" :sm="12">
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
    <ACol :xs="24" :sm="12">
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
        </div>
      </ACard>
    </ACol>
  </ARow>
</template>
