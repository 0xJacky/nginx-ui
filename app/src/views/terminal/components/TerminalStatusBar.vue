<script setup lang="ts">
import type { DiskStat, LoadStat, MemStat } from '@/api/analytic'
import analytic from '@/api/analytic'
import upgrade from '@/api/upgrade'
import { formatDateTime } from '@/lib/helper'

interface StatusData {
  version: string
  uptime: number
  loadAvg: LoadStat | null
  cpuFreq: number
  cpuCount: number
  memory: MemStat | null
  disk: DiskStat | null
  timestamp: string
}

const statusData = ref<StatusData>({
  version: '',
  uptime: 0,
  loadAvg: null,
  cpuFreq: 0,
  cpuCount: 0,
  memory: null,
  disk: null,
  timestamp: '',
})

const websocket = ref<WebSocket | null>(null)

// Format uptime as days and hours
function formatUptime(uptime: number) {
  const days = Math.floor(uptime / (24 * 3600))
  const hours = Math.floor((uptime % (24 * 3600)) / 3600)
  return `${days}d${hours}h`
}

// Format memory usage as "used percentage"
function formatMemoryUsage(memory: MemStat | null) {
  if (!memory)
    return '0B0%'

  // Use the pressure value as percentage (since you said we can get it directly)
  const percentage = memory.pressure.toFixed(1)

  // Remove space from used size and combine without space
  const usedSize = memory.used.replace(' ', '')

  return `${usedSize}${percentage}%`
}

// Format disk usage as "used percentage"
function formatDiskUsage(disk: DiskStat | null) {
  if (!disk)
    return '0B0%'

  // Value is already formatted string like "39 GiB"
  // Use the pre-calculated percentage from the API
  const percentage = disk.percentage.toFixed(1)

  // Remove space from used size and combine without space
  const usedSize = disk.used.replace(' ', '')

  return `${usedSize}${percentage}%`
}

// Format CPU frequency
function formatCpuFreq(freq: number) {
  if (!freq || freq === 0) {
    return 'N/A'
  }
  if (freq >= 1000) {
    return `${(freq / 1000).toFixed(2)}GHz`
  }
  return `${freq.toFixed(0)}MHz`
}

// Update current timestamp
function updateTimestamp() {
  statusData.value.timestamp = formatDateTime(new Date().toISOString())
}

// Initialize data from analytic init API
async function initializeData() {
  try {
    const analyticData = await analytic.init()

    // Set system info with fallbacks
    statusData.value.uptime = analyticData?.host?.uptime || 0
    statusData.value.loadAvg = analyticData?.loadavg || null
    statusData.value.memory = analyticData?.memory || null
    statusData.value.disk = analyticData?.disk || null

    // Set CPU info with fallbacks
    const cpuInfo = analyticData?.cpu?.info || []
    statusData.value.cpuCount = cpuInfo.length || 0

    // Get CPU frequency from first CPU info with fallback
    if (cpuInfo.length > 0 && cpuInfo[0].mhz) {
      statusData.value.cpuFreq = cpuInfo[0].mhz
    }
    else {
      statusData.value.cpuFreq = 0
    }

    // Try to get version from upgrade API, fallback to host platform version
    try {
      const versionData = await upgrade.current_version()
      statusData.value.version = versionData?.cur_version?.version || analyticData?.host?.platformVersion || 'unknown'
    }
    catch (versionError) {
      console.warn('Failed to get app version, using platform version:', versionError)
      statusData.value.version = analyticData?.host?.platformVersion || 'unknown'
    }

    updateTimestamp()
  }
  catch (error) {
    console.error('Failed to initialize terminal status bar:', error)
    // Set default values on error
    statusData.value.version = 'error'
    updateTimestamp()
  }
}

// Connect to WebSocket for real-time updates
function connectWebSocket() {
  try {
    const ws = analytic.server()
    websocket.value = ws as WebSocket

    if (websocket.value) {
      websocket.value.onmessage = event => {
        try {
          const data = JSON.parse(event.data)
          statusData.value.uptime = data.uptime
          statusData.value.loadAvg = data.loadavg
          statusData.value.memory = data.memory
          statusData.value.disk = data.disk
          updateTimestamp()
        }
        catch (error) {
          console.error('Failed to parse WebSocket data:', error)
        }
      }

      websocket.value.onerror = error => {
        console.error('WebSocket error:', error)
      }
    }
  }
  catch (error) {
    console.error('Failed to connect WebSocket:', error)
  }
}

// Cleanup WebSocket connection
function disconnectWebSocket() {
  if (websocket.value) {
    websocket.value.close()
    websocket.value = null
  }
}

onMounted(() => {
  initializeData()
  connectWebSocket()

  // Update timestamp every second
  const timestampInterval = setInterval(updateTimestamp, 1000)

  onUnmounted(() => {
    clearInterval(timestampInterval)
    disconnectWebSocket()
  })
})
</script>

<template>
  <div class="terminal-status-bar">
    <!-- Left side: Version only -->
    <div class="left-section">
      <div class="status-item version">
        <span class="icon i-tabler-package" />
        <span class="value">{{ statusData.version }}</span>
      </div>
    </div>

    <!-- Right side: All system info -->
    <div class="right-section">
      <div class="status-item uptime">
        <span class="icon i-tabler-clock-up" />
        <span class="value">{{ formatUptime(statusData.uptime) }}</span>
      </div>

      <div class="status-item load">
        <span class="icon i-tabler-activity" />
        <span class="value">{{ statusData.loadAvg?.load1.toFixed(2) || '0.00' }}</span>
      </div>

      <div class="status-item cpu">
        <span class="icon i-tabler-cpu" />
        <span class="value">{{ statusData.cpuCount || 0 }}x{{ formatCpuFreq(statusData.cpuFreq || 0) }}</span>
      </div>

      <div class="status-item memory">
        <span class="icon i-tabler-chart-pie" />
        <span class="value">{{ formatMemoryUsage(statusData.memory) }}</span>
      </div>

      <div class="status-item disk">
        <span class="icon i-tabler-database" />
        <span class="value">{{ formatDiskUsage(statusData.disk) }}</span>
      </div>

      <div class="status-item timestamp">
        <span class="icon i-tabler-calendar-time" />
        <span class="value">{{ statusData.timestamp }}</span>
      </div>
    </div>
  </div>
</template>

<style lang="less" scoped>
.terminal-status-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  background: #1a1a1a;
  border-top: 1px solid #333;
  padding: 4px 12px;
  font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
  font-size: 12px;
  height: 28px;
  color: #e0e0e0;
  white-space: nowrap;
  overflow: hidden;

  .left-section,
  .right-section {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .left-section {
    flex-shrink: 0;
  }

  .right-section {
    flex-shrink: 1;
    overflow: hidden;
  }

  .status-item {
    display: flex;
    align-items: center;
    gap: 4px;
    flex-shrink: 0;

    .icon {
      font-size: 14px;
      opacity: 0.8;
      transition: opacity 0.2s;
    }

    .value {
      color: #e0e0e0;
      font-weight: 500;
    }

    &:hover .icon {
      opacity: 1;
    }

    &.version {
      .icon { color: #4a9eff; }
      .value { color: #4a9eff; }
    }

    &.uptime {
      .icon { color: #00d4aa; }
      .value { color: #00d4aa; }
    }

    &.load {
      .icon { color: #ff6b6b; }
      .value { color: #ff6b6b; }
    }

    &.cpu {
      .icon { color: #4ecdc4; }
      .value { color: #4ecdc4; }
    }

    &.memory {
      .icon { color: #ffe66d; }
      .value { color: #ffe66d; }
    }

    &.disk {
      .icon { color: #ff8a65; }
      .value { color: #ff8a65; }
    }

    &.timestamp {
      .icon { color: #b0b0b0; }
      .value {
        color: #b0b0b0;
        font-size: 11px;
      }
    }
  }

  @media (max-width: 768px) {
    padding: 3px 8px;
    font-size: 11px;

    .left-section,
    .right-section {
      gap: 8px;
    }

    .status-item {
      gap: 2px;

      .icon {
        font-size: 12px;
      }

      &.timestamp .value {
        font-size: 10px;
      }
    }
  }

  @media (max-width: 512px) {
    padding: 2px 6px;
    font-size: 10px;

    .left-section,
    .right-section {
      gap: 6px;
    }

    .status-item {
      gap: 1px;

      .icon {
        font-size: 11px;
      }

      &.timestamp .value {
        font-size: 9px;
      }
    }
  }
}
</style>
