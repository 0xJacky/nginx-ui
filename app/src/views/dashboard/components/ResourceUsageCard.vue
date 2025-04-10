<script setup lang="ts">
import type { NginxPerformanceInfo } from '@/api/ngx'
import {
  FundProjectionScreenOutlined,
  InfoCircleOutlined,
  ThunderboltOutlined,
} from '@ant-design/icons-vue'
import { computed, defineProps } from 'vue'

const props = defineProps<{
  nginxInfo: NginxPerformanceInfo
}>()

// 资源利用率
const resourceUtilization = computed(() => {
  const cpuFactor = Math.min(props.nginxInfo.cpu_usage / 100, 1)
  const maxConnections = props.nginxInfo.worker_connections * props.nginxInfo.worker_processes
  const connectionFactor = Math.min(props.nginxInfo.active / maxConnections, 1)

  return Math.round((cpuFactor * 0.5 + connectionFactor * 0.5) * 100)
})
</script>

<template>
  <ACard :title="$gettext('Resource Usage of Nginx')" :bordered="false" class="h-full" :body-style="{ padding: '16px', height: 'calc(100% - 58px)' }">
    <div class="flex flex-col h-full">
      <!-- CPU使用率 -->
      <ARow :gutter="[16, 8]" class="mb-2">
        <ACol :span="24">
          <div class="flex items-center">
            <ThunderboltOutlined class="text-lg mr-2" :style="{ color: nginxInfo.cpu_usage > 80 ? '#cf1322' : '#3f8600' }" />
            <div class="text-base font-medium">
              {{ $gettext('CPU Usage') }}: <span :style="{ color: nginxInfo.cpu_usage > 80 ? '#cf1322' : '#3f8600' }">{{ nginxInfo.cpu_usage.toFixed(2) }}%</span>
            </div>
          </div>
          <AProgress
            :percent="Math.min(nginxInfo.cpu_usage, 100)"
            :format="percent => `${percent?.toFixed(2)}%`"
            :status="nginxInfo.cpu_usage > 80 ? 'exception' : 'active'"
            size="small"
            class="mt-1"
            :show-info="false"
          />
          <div v-if="nginxInfo.cpu_usage > 50" class="text-xs text-orange-500 mt-1">
            {{ $gettext('CPU usage is relatively high, consider optimizing Nginx configuration') }}
          </div>
        </ACol>
      </ARow>

      <!-- 内存使用 -->
      <ARow :gutter="[16, 8]" class="mb-2">
        <ACol :span="24">
          <div class="flex items-center">
            <div class="text-blue-500 text-lg mr-2 flex items-center">
              <FundProjectionScreenOutlined />
            </div>
            <div class="text-base font-medium">
              {{ $gettext('Memory Usage(RSS)') }}: <span class="text-blue-500">{{ nginxInfo.memory_usage.toFixed(2) }} MB</span>
            </div>
            <ATooltip :title="$gettext('Resident Set Size: Actual memory resident in physical memory, including all shared library memory, which will be repeated calculated for multiple processes')">
              <InfoCircleOutlined class="ml-1 text-gray-500" />
            </ATooltip>
          </div>
        </ACol>
      </ARow>

      <div class="mt-1 flex justify-between text-xs text-gray-500">
        {{ $gettext('Per worker memory') }}: {{ (nginxInfo.memory_usage / (nginxInfo.workers || 1)).toFixed(2) }} MB
      </div>

      <!-- 系统负载 -->
      <div class="mt-4 text-xs text-gray-500 border-t border-gray-100 pt-2">
        <div class="flex justify-between mb-1">
          <span>{{ $gettext('System load') }}</span>
          <span class="font-medium">{{ resourceUtilization }}%</span>
        </div>
        <AProgress
          :percent="resourceUtilization"
          size="small"
          :status="resourceUtilization > 80 ? 'exception' : 'active'"
          :stroke-color="resourceUtilization > 80 ? '#ff4d4f' : resourceUtilization > 50 ? '#faad14' : '#52c41a'"
          :show-info="false"
        />
      </div>
    </div>
  </ACard>
</template>
