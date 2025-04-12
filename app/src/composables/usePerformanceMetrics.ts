import type { NginxPerformanceInfo } from '@/api/ngx'

export function usePerformanceMetrics(nginxInfo: Ref<NginxPerformanceInfo | undefined>) {
  // Format numbers to a more readable form
  function formatNumber(num: number): string {
    if (num >= 1000000) {
      return `${(num / 1000000).toFixed(2)}M`
    }
    else if (num >= 1000) {
      return `${(num / 1000).toFixed(2)}K`
    }
    return num.toString()
  }

  // Active connections percentage
  const activeConnectionsPercent = computed(() => {
    if (!nginxInfo.value) {
      return 0
    }
    const maxConnections = nginxInfo.value.worker_connections * nginxInfo.value.worker_processes
    return Number(((nginxInfo.value.active / maxConnections) * 100).toFixed(2))
  })

  // Worker processes usage percentage
  const workerProcessesPercent = computed(() => {
    if (!nginxInfo.value) {
      return 0
    }
    return Number(((nginxInfo.value.workers / nginxInfo.value.worker_processes) * 100).toFixed(2))
  })

  // Requests per connection
  const requestsPerConnection = computed(() => {
    if (!nginxInfo.value || nginxInfo.value.handled === 0) {
      return 0
    }
    return (nginxInfo.value.requests / nginxInfo.value.handled).toFixed(2)
  })

  // Maximum requests per second
  const maxRPS = computed(() => {
    if (!nginxInfo.value) {
      return 0
    }
    return nginxInfo.value.worker_processes * nginxInfo.value.worker_connections
  })

  // Process composition data
  const processTypeData = computed(() => {
    if (!nginxInfo.value) {
      return []
    }

    return [
      { type: $gettext('Worker Processes'), value: nginxInfo.value.workers, color: '#1890ff' },
      { type: $gettext('Master Process'), value: nginxInfo.value.master, color: '#52c41a' },
      { type: $gettext('Cache Processes'), value: nginxInfo.value.cache, color: '#faad14' },
      { type: $gettext('Other Processes'), value: nginxInfo.value.other, color: '#f5222d' },
    ]
  })

  // Resource utilization
  const resourceUtilization = computed(() => {
    if (!nginxInfo.value) {
      return 0
    }

    const cpuFactor = Math.min(nginxInfo.value.cpu_usage / 100, 1)
    const maxConnections = nginxInfo.value.worker_connections * nginxInfo.value.worker_processes
    const connectionFactor = Math.min(nginxInfo.value.active / maxConnections, 1)

    return Math.round((cpuFactor * 0.5 + connectionFactor * 0.5) * 100)
  })

  // Table data
  const statusData = computed(() => {
    if (!nginxInfo.value) {
      return []
    }

    return [
      {
        key: '1',
        name: $gettext('Active connections'),
        value: formatNumber(nginxInfo.value.active),
      },
      {
        key: '2',
        name: $gettext('Total handshakes'),
        value: formatNumber(nginxInfo.value.accepts),
      },
      {
        key: '3',
        name: $gettext('Total connections'),
        value: formatNumber(nginxInfo.value.handled),
      },
      {
        key: '4',
        name: $gettext('Total requests'),
        value: formatNumber(nginxInfo.value.requests),
      },
      {
        key: '5',
        name: $gettext('Read requests'),
        value: nginxInfo.value.reading,
      },
      {
        key: '6',
        name: $gettext('Responses'),
        value: nginxInfo.value.writing,
      },
      {
        key: '7',
        name: $gettext('Waiting processes'),
        value: nginxInfo.value.waiting,
      },
    ]
  })

  // Worker processes data
  const workerData = computed(() => {
    if (!nginxInfo.value) {
      return []
    }

    return [
      {
        key: '1',
        name: $gettext('Number of worker processes'),
        value: nginxInfo.value.workers,
      },
      {
        key: '2',
        name: $gettext('Master process'),
        value: nginxInfo.value.master,
      },
      {
        key: '3',
        name: $gettext('Cache manager processes'),
        value: nginxInfo.value.cache,
      },
      {
        key: '4',
        name: $gettext('Other Nginx processes'),
        value: nginxInfo.value.other,
      },
      {
        key: '5',
        name: $gettext('Nginx CPU usage rate'),
        value: `${nginxInfo.value.cpu_usage.toFixed(2)}%`,
      },
      {
        key: '6',
        name: $gettext('Nginx Memory usage'),
        value: `${nginxInfo.value.memory_usage.toFixed(2)} MB`,
      },
    ]
  })

  // Configuration data
  const configData = computed(() => {
    if (!nginxInfo.value) {
      return []
    }

    return [
      {
        key: '1',
        name: $gettext('Number of worker processes'),
        value: nginxInfo.value.worker_processes,
      },
      {
        key: '2',
        name: $gettext('Maximum number of connections per worker process'),
        value: nginxInfo.value.worker_connections,
      },
    ]
  })

  return {
    formatNumber,
    activeConnectionsPercent,
    workerProcessesPercent,
    requestsPerConnection,
    maxRPS,
    processTypeData,
    resourceUtilization,
    statusData,
    workerData,
    configData,
  }
}
