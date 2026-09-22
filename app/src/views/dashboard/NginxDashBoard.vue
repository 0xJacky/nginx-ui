<script setup lang="ts">
import { ClockCircleOutlined, ReloadOutlined } from '@antdv-next/icons'
import { storeToRefs } from 'pinia'
import ngx from '@/api/ngx'
import { useNginxPerformance } from '@/composables/useNginxPerformance'
import { NginxStatus } from '@/constants'
import { useWebSocket } from '@/lib/websocket'
import { useConnectionSupervisor } from '@/lib/websocket/useConnectionSupervisor'
import { useGlobalStore } from '@/pinia'
import ConnectionMetricsCard from './components/ConnectionMetricsCard.vue'
import ParamsOptimization from './components/ParamsOptimization.vue'
import PerformanceStatisticsCard from './components/PerformanceStatisticsCard.vue'
import PerformanceTablesCard from './components/PerformanceTablesCard.vue'
import ProcessDistributionCard from './components/ProcessDistributionCard.vue'
import ResourceUsageCard from './components/ResourceUsageCard.vue'

// The backend hub pushes a snapshot every 5s, but it collects that snapshot
// synchronously inside the ticker loop, bounded only by the collectors' own
// timeouts (5s stub_status request, 10s container process scan), and ticks
// that fire meanwhile are dropped. A healthy but slow backend can therefore go
// ~20s between frames; only silence beyond that means the connection is gone.
// The server's 30s protocol pings are answered by the browser and never reach
// JavaScript, so they cannot prove liveness here.
const STALE_MESSAGE_MS = 30_000
// How often the supervisor checks the socket; also its base reconnect backoff.
const WATCHDOG_INTERVAL_MS = 10_000
// autoReconnect retries 1s after a drop. A drop that the first retry repairs
// (retry delay plus handshake) should not flash the disconnected warning.
const DISCONNECT_GRACE_MS = 2000

// Global state
const global = useGlobalStore()
const { nginxStatus: status } = storeToRefs(global)

// Use performance data composable
const {
  loading,
  nginxInfo,
  error,
  formattedUpdateTime,
  applyPerformanceData,
  fetchInitialData,
  stubStatusEnabled,
  stubStatusLoading,
  stubStatusError,
} = useNginxPerformance()

let isUnmounted = false

// Set once live data has been interrupted for longer than a blip. Only a new
// frame clears it, so the warning stays until data actually flows again.
const isLiveDataInterrupted = ref(false)
let interruptTimer: ReturnType<typeof setTimeout> | undefined

function clearInterruptTimer() {
  clearTimeout(interruptTimer)
  interruptTimer = undefined
}

function markLiveDataInterrupted() {
  clearInterruptTimer()
  isLiveDataInterrupted.value = true
  loading.value = false
}

function scheduleLiveDataInterrupted() {
  // Nothing will arrive on a closed socket, so never leave the spinner up.
  loading.value = false

  if (isLiveDataInterrupted.value || interruptTimer !== undefined) {
    return
  }

  interruptTimer = setTimeout(markLiveDataInterrupted, DISCONNECT_GRACE_MS)
}

// One socket for the lifetime of the page. autoReconnect covers short blips;
// the supervisor below covers everything autoReconnect gives up on.
const socket = useWebSocket('/api/nginx/detail_status/ws', true, {
  immediate: false,
  onConnected() {
    clearInterruptTimer()
  },
  onDisconnected(ws) {
    // open() and close() detach the previous socket before its close event
    // fires; only a drop of the current socket means live data stopped.
    if (ws !== socket.ws.value) {
      return
    }

    scheduleLiveDataInterrupted()
  },
  onMessage(_, event) {
    clearInterruptTimer()
    isLiveDataInterrupted.value = false
    loading.value = false

    try {
      applyPerformanceData(JSON.parse(event.data))
    }
    catch (parseError) {
      console.error('Error parsing WebSocket message:', parseError)
    }
  },
})

function dial() {
  try {
    // Closes the previous socket and cancels any queued retry before dialing.
    socket.open()
  }
  catch (err) {
    console.error('Failed to create WebSocket connection:', err)
    // The supervisor keeps retrying with backoff, so the warning stays truthful.
    markLiveDataInterrupted()
  }
}

// Recovers what autoReconnect cannot: retries after its budget is spent (with
// backoff, and immediately when the tab becomes visible or the network
// returns), and rebuilds a half-open socket that still reads OPEN but has
// stopped delivering.
const supervisor = useConnectionSupervisor({
  socket: {
    ws: socket.ws,
    // Only used to rebuild a socket that has been silent past STALE_MESSAGE_MS,
    // so the data on screen is already stale.
    open: () => {
      markLiveDataInterrupted()
      dial()
    },
  },
  // Only used when the socket is neither open nor connecting.
  connect: () => {
    scheduleLiveDataInterrupted()
    dial()
  },
  staleAfterMs: STALE_MESSAGE_MS,
  intervalMs: WATCHDOG_INTERVAL_MS,
})

// Toggle stub_status module status
async function toggleStubStatus() {
  try {
    stubStatusLoading.value = true
    stubStatusError.value = ''
    const response = await ngx.toggle_stub_status(!stubStatusEnabled.value)

    if (response.stub_status_enabled !== undefined) {
      stubStatusEnabled.value = response.stub_status_enabled
    }

    if (response.error) {
      stubStatusError.value = response.error
    }
    else {
      fetchInitialData().then(connectWebSocket)
    }
  }
  catch (err) {
    console.error('Toggle stub_status failed:', err)
    stubStatusError.value = $gettext('Toggle failed')
  }
  finally {
    stubStatusLoading.value = false
  }
}

// Connect WebSocket. Safe to call repeatedly (refresh, stub_status toggle):
// the page reuses one socket and the supervisor keeps a single interval.
function connectWebSocket() {
  // fetchInitialData() may resolve after the user has already left the page.
  if (isUnmounted) {
    return
  }

  loading.value = true
  // A user-driven reconnect starts from a clean backoff.
  supervisor.stop()
  dial()
  supervisor.start()
}

// Disconnect WebSocket
function disconnectWebSocket() {
  supervisor.stop()
  clearInterruptTimer()
  // close() also cancels a queued autoReconnect retry, so call it even when
  // the socket already reads CLOSED.
  socket.close()
}

// Manually refresh data
function refreshData() {
  fetchInitialData().then(connectWebSocket)
}

// Initialize connection when the component is mounted
onMounted(() => {
  fetchInitialData().then(connectWebSocket)
})

// Clean up WebSocket connection when component is unmounted
onUnmounted(() => {
  isUnmounted = true
  disconnectWebSocket()
})
</script>

<template>
  <div class="max-w-full of-x-hidden">
    <!-- Top operation bar -->
    <div class="mb-4 mx-6 md:mx-0 flex flex-wrap gap-4 justify-between items-center">
      <ABadge :status="status === NginxStatus.Running ? 'success' : 'error'">
        <template #text>
          <span class="font-medium">{{ status === NginxStatus.Running ? $gettext('Nginx is running') : $gettext('Nginx is not running') }}</span>
        </template>
      </ABadge>
      <div class="flex items-center">
        <ClockCircleOutlined class="mr-1 text-gray-500" />
        <span class="mr-4 text-gray-500 text-sm text-nowrap">{{ $gettext('Last update') }}: {{ formattedUpdateTime }}</span>
        <AButton type="text" size="small" :loading="loading" @click="refreshData">
          <template #icon>
            <ReloadOutlined />
          </template>
        </AButton>
      </div>
    </div>

    <!-- Live data interruption prompt -->
    <AAlert
      v-if="isLiveDataInterrupted"
      class="mb-4"
      type="warning"
      show-icon
      :title="$gettext('Disconnected')"
      :description="$gettext('Connection error, trying to reconnect...')"
    />

    <!-- Nginx status prompt -->
    <AAlert
      v-if="status !== NginxStatus.Running"
      class="mb-4"
      type="warning"
      show-icon
      :title="$gettext('Nginx is not running')"
      :description="$gettext('Cannot get performance data in this state')"
    />

    <!-- Error prompt -->
    <AAlert
      v-if="error"
      class="mb-4"
      type="error"
      show-icon
      :title="$gettext('Get data failed')"
      :description="error"
    />

    <!-- stub_status switch -->
    <ACard class="mb-4" variant="borderless">
      <div class="flex items-center justify-between">
        <div>
          <div class="font-medium mb-1">
            {{ $gettext('Enable stub_status module') }}
          </div>
          <div class="text-gray-500 text-sm">
            {{ $gettext('This module provides Nginx request statistics, connection count, etc. data. After enabling it, you can view performance statistics') }}
          </div>
          <div v-if="stubStatusError" class="text-red-500 text-sm mt-1">
            {{ stubStatusError }}
          </div>
        </div>
        <ASwitch
          :checked="stubStatusEnabled"
          :loading="stubStatusLoading"
          @change="toggleStubStatus"
        />
      </div>
    </ACard>

    <!-- stub_status module is not enabled -->
    <AAlert
      v-if="status === NginxStatus.Running && !stubStatusEnabled && !error"
      class="mb-4"
      type="info"
      show-icon
      :title="$gettext('Need to enable the stub_status module')"
      :description="$gettext('Please enable the stub_status module to get request statistics, connection count, etc.')"
    />

    <!-- Loading state -->
    <ASpin :spinning="loading" :description="$gettext('Loading data...')">
      <div v-if="!nginxInfo && !error" class="text-center py-8">
        <AEmpty :description="$gettext('No data')" />
      </div>

      <div v-if="nginxInfo" class="performance-dashboard">
        <!-- Top performance metrics card -->
        <ACard class="mb-4" :title="$gettext('Performance Metrics')" variant="borderless">
          <template #extra>
            <ParamsOptimization />
          </template>
          <PerformanceStatisticsCard :nginx-info="nginxInfo" />
        </ACard>

        <ARow :gutter="[16, 16]" class="mb-4">
          <!-- Metrics card -->
          <ACol :xs="24" :sm="24" :lg="12">
            <ConnectionMetricsCard :nginx-info="nginxInfo" />
          </ACol>

          <!-- CPU and memory usage -->
          <ACol :xs="24" :sm="24" :lg="12">
            <ResourceUsageCard :nginx-info="nginxInfo" />
          </ACol>
        </ARow>

        <!-- Resource monitoring -->
        <ARow :gutter="[16, 16]" class="mb-4">
          <!-- Process distribution -->
          <ACol :span="24">
            <ProcessDistributionCard :nginx-info="nginxInfo" />
          </ACol>
        </ARow>

        <!-- Performance metrics table -->
        <ARow :gutter="[16, 16]" class="mb-4">
          <ACol :span="24">
            <PerformanceTablesCard :nginx-info="nginxInfo" />
          </ACol>
        </ARow>
      </div>
    </ASpin>
  </div>
</template>
