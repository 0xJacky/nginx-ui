<script setup lang="ts">
import type { NginxPerformanceInfo } from '@/api/ngx'
import { InfoCircleOutlined } from '@ant-design/icons-vue'

const props = defineProps<{
  nginxInfo: NginxPerformanceInfo
}>()

// Process composition data
const processTypeData = computed(() => {
  return [
    { type: $gettext('Worker Processes'), value: props.nginxInfo.workers, color: '#1890ff' },
    { type: $gettext('Master Process'), value: props.nginxInfo.master, color: '#52c41a' },
    { type: $gettext('Cache Processes'), value: props.nginxInfo.cache, color: '#faad14' },
    { type: $gettext('Other Processes'), value: props.nginxInfo.other, color: '#f5222d' },
  ]
})

// Total processes
const totalProcesses = computed(() => {
  return props.nginxInfo.workers + props.nginxInfo.master + props.nginxInfo.cache + props.nginxInfo.other
})
</script>

<template>
  <ACard :title="$gettext('Process Distribution')" :bordered="false" class="h-full" :body-style="{ height: 'calc(100% - 58px)' }">
    <div class="process-distribution h-full flex flex-col justify-between">
      <div>
        <div v-for="(item, index) in processTypeData" :key="index" class="mb-3">
          <div class="flex items-center">
            <div class="w-3 h-3 rounded-full mr-2" :style="{ backgroundColor: item.color }" />
            <div class="flex-grow truncate">
              {{ item.type }}
            </div>
            <div class="font-medium w-8 text-right">
              {{ item.value }}
            </div>
          </div>
          <AProgress
            :percent="totalProcesses === 0 ? 0 : (item.value / totalProcesses) * 100"
            :stroke-color="item.color"
            size="small"
            :show-info="false"
          />
        </div>
      </div>
      <div class="mt-auto text-xs text-gray-500 truncate">
        {{ $gettext('Actual worker to configured ratio') }}:
        <span class="font-medium">{{ nginxInfo.workers }} / {{ nginxInfo.worker_processes }}</span>
      </div>

      <div class="mt-2 text-xs text-gray-500 overflow-hidden text-ellipsis">
        {{ $gettext('Total Nginx processes') }}: {{ nginxInfo.workers + nginxInfo.master + nginxInfo.cache + nginxInfo.other }}
        <ATooltip :title="$gettext('Includes master process, worker processes, cache processes, and other Nginx processes')">
          <InfoCircleOutlined class="ml-1" />
        </ATooltip>
      </div>
    </div>
  </ACard>
</template>
