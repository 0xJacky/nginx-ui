<script setup lang="ts">
import type ReconnectingWebSocket from 'reconnecting-websocket'
import { ClockCircleOutlined, ReloadOutlined } from '@ant-design/icons-vue'
import { storeToRefs } from 'pinia'
import ngx from '@/api/ngx'
import { useNginxPerformance } from '@/composables/useNginxPerformance'
import { NginxStatus } from '@/constants'
import ws from '@/lib/websocket'
import { useGlobalStore } from '@/pinia'
import ConnectionMetricsCard from './components/ConnectionMetricsCard.vue'
import ParamsOptimization from './components/ParamsOptimization.vue'
import PerformanceStatisticsCard from './components/PerformanceStatisticsCard.vue'
import PerformanceTablesCard from './components/PerformanceTablesCard.vue'
import ProcessDistributionCard from './components/ProcessDistributionCard.vue'
import ResourceUsageCard from './components/ResourceUsageCard.vue'

// Global state
const global = useGlobalStore()
const { nginxStatus: status } = storeToRefs(global)

// Use performance data composable
const {
  loading,
  nginxInfo,
  error,
  formattedUpdateTime,
  updateLastUpdateTime,
  fetchInitialData,
  stubStatusEnabled,
  stubStatusLoading,
  stubStatusError,
} = useNginxPerformance()

// WebSocket connection
const wsInstance = shallowRef<WebSocket | ReconnectingWebSocket | null>(null)

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

// Connect WebSocket
function connectWebSocket() {
  disconnectWebSocket()
  loading.value = true

  try {
    const wsConnection = ws('api/nginx/detail_status/ws')
    wsInstance.value = wsConnection

    wsConnection.onmessage = event => {
      loading.value = false

      try {
        const data = JSON.parse(event.data)

        if (data.running) {
          nginxInfo.value = data.info
          updateLastUpdateTime()
        }
        else {
          error.value = data.message || $gettext('Nginx is not running')
        }

        if (data.error) {
          error.value = data.error
        }

        stubStatusEnabled.value = data.stub_status_enabled
      }
      catch (parseError) {
        console.error('Error parsing WebSocket message:', parseError)
      }
    }
  }
  catch (err) {
    console.error('Failed to create WebSocket connection:', err)
    error.value = $gettext('Connection error, trying to reconnect...')
  }
}

// Disconnect WebSocket
function disconnectWebSocket() {
  if (wsInstance.value) {
    wsInstance.value.close()
    wsInstance.value = null
  }
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
  disconnectWebSocket()
})
</script>

<template>
  <div class="max-w-full of-x-hidden">
    <!-- Top operation bar -->
    <div class="mb-4 mx-6 md:mx-0 flex flex-wrap gap-4 justify-between items-center">
      <div class="flex items-center">
        <ABadge :status="status === NginxStatus.Running ? 'success' : 'error'" />
        <span class="font-medium">{{ status === NginxStatus.Running ? $gettext('Nginx is running') : $gettext('Nginx is not running') }}</span>
      </div>
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

    <!-- Nginx status prompt -->
    <AAlert
      v-if="status !== NginxStatus.Running"
      class="mb-4"
      type="warning"
      show-icon
      :message="$gettext('Nginx is not running')"
      :description="$gettext('Cannot get performance data in this state')"
    />

    <!-- Error prompt -->
    <AAlert
      v-if="error"
      class="mb-4"
      type="error"
      show-icon
      :message="$gettext('Get data failed')"
      :description="error"
    />

    <!-- stub_status switch -->
    <ACard class="mb-4" :bordered="false">
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
      :message="$gettext('Need to enable the stub_status module')"
      :description="$gettext('Please enable the stub_status module to get request statistics, connection count, etc.')"
    />

    <!-- Loading state -->
    <ASpin :spinning="loading" :tip="$gettext('Loading data...')">
      <div v-if="!nginxInfo && !error" class="text-center py-8">
        <AEmpty :description="$gettext('No data')" />
      </div>

      <div v-if="nginxInfo" class="performance-dashboard">
        <!-- Top performance metrics card -->
        <ACard class="mb-4" :title="$gettext('Performance Metrics')" :bordered="false">
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
