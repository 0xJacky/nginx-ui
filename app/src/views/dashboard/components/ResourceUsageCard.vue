<script setup lang="ts">
import type { NginxPerformanceInfo } from '@/api/ngx'
import {
  FundProjectionScreenOutlined,
  InfoCircleOutlined,
  ThunderboltOutlined,
} from '@ant-design/icons-vue'

const props = defineProps<{
  nginxInfo: NginxPerformanceInfo
}>()

const cpuUsage = computed(() => {
  return Number(Math.min(props.nginxInfo.cpu_usage, 100).toFixed(2))
})
</script>

<template>
  <ACard :bordered="false" class="h-full" :body-style="{ padding: '20px', height: 'calc(100% - 58px)' }">
    <div class="flex flex-col h-full">
      <!-- CPU usage -->
      <ARow :gutter="[16, 8]">
        <ACol :span="24">
          <div class="flex items-center">
            <ThunderboltOutlined class="text-lg mr-2" :style="{ color: nginxInfo.cpu_usage > 80 ? '#cf1322' : '#3f8600' }" />
            <div class="text-base font-medium">
              {{ $gettext('CPU Usage') }}: <span :style="{ color: nginxInfo.cpu_usage > 80 ? '#cf1322' : '#3f8600' }">{{ cpuUsage.toFixed(2) }}%</span>
            </div>
          </div>
          <AProgress
            :percent="cpuUsage"
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

      <!-- Memory usage -->
      <ARow :gutter="[16, 8]" class="mt-2">
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
    </div>
  </ACard>
</template>
