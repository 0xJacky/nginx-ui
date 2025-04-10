<script setup lang="ts">
import { useNginxPerformance } from '@/composables/useNginxPerformance'
import { useSSE } from '@/composables/useSSE'
import { NginxStatus } from '@/constants'
import { useUserStore } from '@/pinia'
import { useGlobalStore } from '@/pinia/moudule/global'
import { ClockCircleOutlined, ReloadOutlined } from '@ant-design/icons-vue'
import { storeToRefs } from 'pinia'
import ConnectionMetricsCard from './components/ConnectionMetricsCard.vue'
import PerformanceStatisticsCard from './components/PerformanceStatisticsCard.vue'
import PerformanceTablesCard from './components/PerformanceTablesCard.vue'
import ProcessDistributionCard from './components/ProcessDistributionCard.vue'
import ResourceUsageCard from './components/ResourceUsageCard.vue'

// Global state
const global = useGlobalStore()
const { nginxStatus: status } = storeToRefs(global)
const { token } = storeToRefs(useUserStore())

// Use performance data composable
const {
  loading,
  nginxInfo,
  error,
  formattedUpdateTime,
  updateLastUpdateTime,
  fetchInitialData,
} = useNginxPerformance()

// SSE connection
const { connect, disconnect } = useSSE()

// Connect SSE
function connectSSE() {
  disconnect()
  loading.value = true

  connect({
    url: 'api/nginx/detailed_status/stream',
    token: token.value,
    onMessage: data => {
      loading.value = false

      if (data.running) {
        nginxInfo.value = data.info
        updateLastUpdateTime()
      }
      else {
        error.value = data.message || $gettext('Nginx is not running')
      }
    },
    onError: () => {
      error.value = $gettext('Connection error, trying to reconnect...')

      // If the connection fails, try to get data using the traditional method
      setTimeout(() => {
        fetchInitialData()
      }, 5000)
    },
  })
}

// Manually refresh data
function refreshData() {
  fetchInitialData().then(connectSSE)
}

// Initialize connection when the component is mounted
onMounted(() => {
  fetchInitialData().then(connectSSE)
})
</script>

<template>
  <div>
    <!-- Top operation bar -->
    <div class="mb-4 mx-6 md:mx-0 flex flex-wrap justify-between items-center">
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

    <!-- Loading state -->
    <ASpin :spinning="loading" :tip="$gettext('Loading data...')">
      <div v-if="!nginxInfo && !error" class="text-center py-8">
        <AEmpty :description="$gettext('No data')" />
      </div>

      <div v-if="nginxInfo" class="performance-dashboard">
        <!-- Top performance metrics card -->
        <ARow :gutter="[16, 16]" class="mb-4">
          <ACol :span="24">
            <ACard :title="$gettext('Performance Metrics')" :bordered="false">
              <PerformanceStatisticsCard :nginx-info="nginxInfo" />
            </ACard>
          </ACol>
        </ARow>

        <!-- Metrics card -->
        <ConnectionMetricsCard :nginx-info="nginxInfo" class="mb-4" />

        <!-- Resource monitoring -->
        <ARow :gutter="[16, 16]" class="mb-4">
          <!-- CPU and memory usage -->
          <ACol :xs="24" :md="12">
            <ResourceUsageCard :nginx-info="nginxInfo" />
          </ACol>
          <!-- Process distribution -->
          <ACol :xs="24" :md="12">
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
