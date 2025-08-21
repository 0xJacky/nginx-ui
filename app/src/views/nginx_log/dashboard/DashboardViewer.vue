<script setup lang="ts">
import type { AnalyticsRequest, ChinaMapData, DashboardAnalytics, DashboardRequest, WorldMapData } from '@/api/nginx_log'
import { LoadingOutlined } from '@ant-design/icons-vue'
import { Col, Row } from 'ant-design-vue'
import dayjs from 'dayjs'
import nginx_log from '@/api/nginx_log'
import BrowserStatsTable from './components/BrowserStatsTable.vue'
import DailyTrendsChart from './components/DailyTrendsChart.vue'
import DateRangeSelector from './components/DateRangeSelector.vue'
import DeviceStatsTable from './components/DeviceStatsTable.vue'
import GeoMapChart from './components/GeoMapChart.vue'
import HourlyChart from './components/HourlyChart.vue'
import OSStatsTable from './components/OSStatsTable.vue'
import SummaryStats from './components/SummaryStats.vue'
import TopUrlsTable from './components/TopUrlsTable.vue'

// Props
const props = defineProps<{
  logPath: string
}>()

// Reactive data
const loading = ref(true)
const dashboardData = ref<DashboardAnalytics | null>(null)
const dateRange = ref<[dayjs.Dayjs, dayjs.Dayjs]>([
  dayjs().subtract(7, 'day'), // Default fallback
  dayjs(),
])
const timeRangeLoaded = ref(false)

// Geographic data
const worldMapData = ref<WorldMapData[] | null>(null)
const chinaMapData = ref<ChinaMapData[] | null>(null)
const geoLoading = ref(false)

// Overall loading state for refresh button
const refreshLoading = computed(() => loading.value || geoLoading.value)

// Load time range from preflight API for specific log file
async function loadTimeRange() {
  if (timeRangeLoaded.value)
    return

  try {
    const preflight = await nginx_log.getPreflight(props.logPath)

    if (preflight.available && preflight.start_time && preflight.end_time) {
      const endTime = dayjs.unix(preflight.end_time)

      // Use last week's data as default range (from last day back to 7 days ago)
      const weekStart = endTime.subtract(7, 'day').startOf('day')
      const lastDayEnd = endTime.endOf('day')
      dateRange.value = [weekStart, lastDayEnd]
      timeRangeLoaded.value = true

      // Time range loaded successfully
    }
    else {
      console.warn(`No valid time range available for ${props.logPath}, using default range`)
    }
  }
  catch (error) {
    console.error('Failed to load time range from preflight:', error)
  }
}

// Load dashboard data for specific log file
async function loadDashboardData() {
  loading.value = true
  try {
    const request: DashboardRequest = {
      log_path: props.logPath,
      start_date: dateRange.value[0].format('YYYY-MM-DD'),
      end_date: dateRange.value[1].format('YYYY-MM-DD'),
    }

    dashboardData.value = await nginx_log.getDashboardAnalytics(request)
  }
  catch (error) {
    console.error('Failed to load dashboard data:', error)
    dashboardData.value = null
  }
  finally {
    loading.value = false
  }
}

// Load geographic data
async function loadGeographicData() {
  geoLoading.value = true
  try {
    const request: AnalyticsRequest = {
      path: props.logPath,
      start_time: dateRange.value[0].unix(),
      end_time: dateRange.value[1].unix(),
    }

    // Load both world and China map data in parallel
    const [worldResponse, chinaResponse] = await Promise.all([
      nginx_log.getWorldMapData(request),
      nginx_log.getChinaMapData(request),
    ])

    worldMapData.value = worldResponse.data
    chinaMapData.value = chinaResponse.data
  }
  catch (error) {
    console.error('Failed to load geographic data:', error)
    worldMapData.value = null
    chinaMapData.value = null
  }
  finally {
    geoLoading.value = false
  }
}

// Refresh all dashboard data
function refreshAllData() {
  Promise.all([loadDashboardData(), loadGeographicData()])
}

// Initialize time range when log path changes
watch(() => props.logPath, async () => {
  timeRangeLoaded.value = false
  const oldDateRange = dateRange.value
  loadTimeRange()

  // Only load dashboard data if dateRange didn't change (no automatic trigger)
  if (timeRangeLoaded.value
    && oldDateRange[0].isSame(dateRange.value[0])
    && oldDateRange[1].isSame(dateRange.value[1])) {
    await Promise.all([loadDashboardData(), loadGeographicData()])
  }
}, { immediate: true })

// Reload data when date range changes (after initial load)
watch(dateRange, () => {
  if (timeRangeLoaded.value) {
    Promise.all([loadDashboardData(), loadGeographicData()])
  }
}, { deep: true })
</script>

<template>
  <div class="dashboard-viewer">
    <!-- Loading Spinner -->
    <div v-if="loading" class="text-center" style="padding: 40px;">
      <LoadingOutlined class="text-2xl text-blue-500" />
      <p style="margin-top: 16px;">
        {{ $gettext('Loading dashboard data...') }}
      </p>
    </div>

    <!-- Dashboard Content -->
    <div v-else>
      <!-- Date Range Selector -->
      <DateRangeSelector
        v-model:date-range="dateRange"
        :log-path="logPath"
        :refresh-loading="refreshLoading"
        @refresh="refreshAllData"
      />

      <!-- Summary Statistics -->
      <SummaryStats :dashboard-data="dashboardData" />

      <!-- Charts Row -->
      <Row :gutter="16" class="mb-4">
        <!-- 24-Hour UV/PV Bar Chart -->
        <Col :span="12">
          <HourlyChart
            :dashboard-data="dashboardData"
            :loading="loading"
            :end-date="dateRange[1].format('YYYY-MM-DD')"
          />
        </Col>

        <!-- Daily Trends Area Chart -->
        <Col :span="12">
          <DailyTrendsChart :dashboard-data="dashboardData" :loading="loading" />
        </Col>
      </Row>

      <!-- Geographic Visualizations -->
      <Row :gutter="16" class="mb-4">
        <Col :span="24">
          <GeoMapChart
            :world-data="worldMapData"
            :china-data="chinaMapData"
            :loading="geoLoading"
          />
        </Col>
      </Row>

      <!-- TOP 10 URLs Table -->
      <TopUrlsTable :dashboard-data="dashboardData" :loading="loading" />

      <!-- Browser, OS, Device Statistics -->
      <Row :gutter="16">
        <!-- Browser Statistics -->
        <Col :span="8">
          <BrowserStatsTable :dashboard-data="dashboardData" :loading="loading" />
        </Col>

        <!-- Operating System Statistics -->
        <Col :span="8">
          <OSStatsTable :dashboard-data="dashboardData" :loading="loading" />
        </Col>

        <!-- Device Statistics -->
        <Col :span="8">
          <DeviceStatsTable :dashboard-data="dashboardData" :loading="loading" />
        </Col>
      </Row>
    </div>
  </div>
</template>

<style scoped>
.dashboard-viewer {
  padding: 0;
}

/* Responsive adjustments */
@media (max-width: 1200px) {
  .dashboard-viewer :deep(.ant-col) {
    margin-bottom: 16px;
  }
}

@media (max-width: 768px) {
  .dashboard-viewer :deep(.ant-row) {
    flex-direction: column;
  }

  .dashboard-viewer :deep(.ant-col) {
    width: 100% !important;
    max-width: 100% !important;
  }
}
</style>
