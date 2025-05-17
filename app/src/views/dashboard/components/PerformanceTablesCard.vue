<script setup lang="tsx">
import type { TableColumnType } from 'ant-design-vue'
import type { NginxPerformanceInfo } from '@/api/ngx'
import { InfoCircleOutlined } from '@ant-design/icons-vue'
import ModulesTable from './ModulesTable.vue'

const props = defineProps<{
  nginxInfo: NginxPerformanceInfo
}>()

const activeTabKey = ref('status')

// Table column definition
const columns: TableColumnType[] = [
  {
    title: $gettext('Indicator'),
    dataIndex: 'name',
    key: 'name',
    width: '30%',
  },
  {
    title: $gettext('Value'),
    dataIndex: 'value',
    key: 'value',
  },
]

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

// Status data
const statusData = computed(() => {
  return [
    {
      key: '1',
      name: $gettext('Active connections'),
      value: formatNumber(props.nginxInfo.active),
    },
    {
      key: '2',
      name: $gettext('Total handshakes'),
      value: formatNumber(props.nginxInfo.accepts),
    },
    {
      key: '3',
      name: $gettext('Total connections'),
      value: formatNumber(props.nginxInfo.handled),
    },
    {
      key: '4',
      name: $gettext('Total requests'),
      value: formatNumber(props.nginxInfo.requests),
    },
    {
      key: '5',
      name: $gettext('Read requests'),
      value: props.nginxInfo.reading,
    },
    {
      key: '6',
      name: $gettext('Responses'),
      value: props.nginxInfo.writing,
    },
    {
      key: '7',
      name: $gettext('Waiting processes'),
      value: props.nginxInfo.waiting,
    },
  ]
})

// Worker processes data
const workerData = computed(() => {
  return [
    {
      key: '1',
      name: $gettext('Number of worker processes'),
      value: props.nginxInfo.workers,
    },
    {
      key: '2',
      name: $gettext('Master process'),
      value: props.nginxInfo.master,
    },
    {
      key: '3',
      name: $gettext('Cache manager processes'),
      value: props.nginxInfo.cache,
    },
    {
      key: '4',
      name: $gettext('Other Nginx processes'),
      value: props.nginxInfo.other,
    },
    {
      key: '5',
      name: $gettext('Nginx CPU usage rate'),
      value: `${props.nginxInfo.cpu_usage.toFixed(2)}%`,
    },
    {
      key: '6',
      name: $gettext('Nginx Memory usage'),
      value: `${props.nginxInfo.memory_usage.toFixed(2)} MB`,
    },
  ]
})

// Configuration data
const configData = computed(() => {
  return [
    {
      key: '1',
      name: $gettext('Number of worker processes'),
      value: props.nginxInfo.worker_processes,
    },
    {
      key: '2',
      name: $gettext('Maximum number of connections per worker process'),
      value: props.nginxInfo.worker_connections,
    },
  ]
})

// Maximum requests per second
const maxRPS = computed(() => {
  return props.nginxInfo.worker_processes * props.nginxInfo.worker_connections
})
</script>

<template>
  <ACard :bordered="false">
    <ATabs v-model:active-key="activeTabKey">
      <!-- Request statistics -->
      <ATabPane key="status" :tab="$gettext('Request statistics')">
        <div class="overflow-x-auto">
          <ATable
            :columns="columns"
            :data-source="statusData"
            :pagination="false"
            size="middle"
            :scroll="{ x: '100%' }"
          />
        </div>
      </ATabPane>

      <!-- Process information -->
      <ATabPane key="workers" :tab="$gettext('Process information')">
        <div class="overflow-x-auto">
          <ATable
            :columns="columns"
            :data-source="workerData"
            :pagination="false"
            size="middle"
            :scroll="{ x: '100%' }"
          />
        </div>
      </ATabPane>

      <!-- Configuration information -->
      <ATabPane key="config" :tab="$gettext('Configuration information')">
        <div class="overflow-x-auto">
          <ATable
            :columns="columns"
            :data-source="configData"
            :pagination="false"
            size="middle"
            :scroll="{ x: '100%' }"
          />
        </div>
        <div class="mt-4">
          <AAlert type="info" show-icon>
            <template #message>
              {{ $gettext('Nginx theoretical maximum performance') }}
            </template>
            <template #description>
              <p>
                {{ $gettext('Theoretical maximum concurrent connections:') }}
                <strong>{{ nginxInfo.worker_processes * nginxInfo.worker_connections }}</strong>
              </p>
              <p>
                {{ $gettext('Theoretical maximum RPS (Requests Per Second):') }}
                <strong>{{ maxRPS }}</strong>
                <ATooltip :title="$gettext('Calculated based on worker_processes * worker_connections. Actual performance depends on hardware, configuration, and workload')">
                  <InfoCircleOutlined class="ml-1 text-gray-500" />
                </ATooltip>
              </p>
              <p>
                {{ $gettext('Maximum worker process number:') }}
                <strong>{{ nginxInfo.worker_processes }}</strong>
                <span class="text-gray-500 text-xs ml-2">
                  {{
                    nginxInfo.process_mode === 'auto'
                      ? $gettext('auto = CPU cores')
                      : $gettext('manually set')
                  }}
                </span>
              </p>
              <p class="mb-0">
                {{ $gettext('Tips: You can increase the concurrency processing capacity by increasing worker_processes or worker_connections') }}
              </p>
            </template>
          </AAlert>
        </div>
      </ATabPane>

      <!-- Modules information -->
      <ATabPane key="modules" :tab="$gettext('Modules')">
        <ModulesTable />
      </ATabPane>
    </ATabs>
  </ACard>
</template>

<style lang="less" scoped>
:deep(.ant-tabs-tab) {
  padding-top: 0;
}
</style>
