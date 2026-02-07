<script setup lang="ts">
import type { DashboardAnalytics, HourlyStats } from '@/api/nginx_log'
import { storeToRefs } from 'pinia'
import VueApexchart from 'vue3-apexcharts'
import { useSettingsStore } from '@/pinia'

const props = defineProps<{
  dashboardData: DashboardAnalytics | null
  loading: boolean
  endDate?: string
}>()

const settings = useSettingsStore()
const { theme } = storeToRefs(settings)

function fontColor() {
  return theme.value === 'dark' ? '#b4b4b4' : undefined
}

const hourlyChartOptions = computed(() => {
  if (!props.dashboardData || !props.dashboardData.hourly_stats)
    return {}

  // Filter hourly data to get only the 24 hours for the end_date in local timezone
  const allHourlyData = props.dashboardData.hourly_stats || []

  // Get the end date in local timezone
  const endDateLocal = props.endDate ? new Date(`${props.endDate}T00:00:00`) : new Date()
  const startOfDayTimestamp = Math.floor(endDateLocal.getTime() / 1000)
  const endOfDayTimestamp = startOfDayTimestamp + (24 * 60 * 60)

  // Filter data for the local date's 24 hours
  const hourlyData = allHourlyData.filter(item =>
    item.timestamp >= startOfDayTimestamp && item.timestamp < endOfDayTimestamp,
  )

  // Sort by timestamp and ensure we have 24 hours
  hourlyData.sort((a, b) => a.timestamp - b.timestamp)

  // Create final data with proper hour values (0-23) based on local time
  const finalHourlyData: HourlyStats[] = []
  for (let hour = 0; hour < 24; hour++) {
    const targetTimestamp = startOfDayTimestamp + hour * 3600
    const found = hourlyData.find(item =>
      item.timestamp >= targetTimestamp && item.timestamp < targetTimestamp + 3600,
    )
    if (found) {
      finalHourlyData.push({ ...found, hour })
    }
    else {
      finalHourlyData.push({ hour, uv: 0, pv: 0, timestamp: targetTimestamp })
    }
  }

  const hours = finalHourlyData.map(item => `${item.hour}`)

  return {
    chart: {
      type: 'bar',
      height: 300,
      toolbar: {
        show: false,
      },
    },
    title: {
      text: props.endDate
        ? `${$gettext('24-Hour UV/PV Statistics')} (${props.endDate})`
        : $gettext('24-Hour UV/PV Statistics'),
      align: 'center',
      style: {
        fontSize: '14px',
        color: fontColor(),
      },
    },
    colors: ['#1890ff', '#52c41a'], // PV蓝色, UV绿色
    dataLabels: {
      enabled: false,
    },
    stroke: {
      show: true,
      width: 2,
      colors: ['transparent'],
    },
    xaxis: {
      categories: hours,
      title: {
        text: $gettext('Hour'),
        style: {
          color: fontColor(),
        },
      },
      labels: {
        style: {
          colors: fontColor(),
        },
      },
    },
    yaxis: {
      title: {
        text: $gettext('Count'),
        style: {
          color: fontColor(),
        },
      },
      labels: {
        style: {
          colors: fontColor(),
        },
        formatter(val: number) {
          return val.toLocaleString()
        },
      },
    },
    fill: {
      opacity: 1,
    },
    tooltip: {
      theme: theme.value === 'dark' ? 'dark' : 'light',
      y: {
        formatter(val: number) {
          return val.toLocaleString()
        },
      },
    },
    legend: {
      position: 'top',
      horizontalAlign: 'center',
      labels: {
        colors: fontColor(),
      },
    },
  }
})

const hourlySeries = computed(() => {
  if (!props.dashboardData || !props.dashboardData.hourly_stats)
    return []

  // Use the same filtered data as in hourlyChartOptions
  const allHourlyData = props.dashboardData.hourly_stats || []

  // Get the end date in local timezone
  const endDateLocal = props.endDate ? new Date(`${props.endDate}T00:00:00`) : new Date()
  const startOfDayTimestamp = Math.floor(endDateLocal.getTime() / 1000)
  const endOfDayTimestamp = startOfDayTimestamp + (24 * 60 * 60)

  // Filter data for the local date's 24 hours
  const hourlyData = allHourlyData.filter(item =>
    item.timestamp >= startOfDayTimestamp && item.timestamp < endOfDayTimestamp,
  )

  // Sort by timestamp and ensure we have 24 hours
  hourlyData.sort((a, b) => a.timestamp - b.timestamp)

  // Create final data with proper hour values (0-23) based on local time
  const finalHourlyData: HourlyStats[] = []
  for (let hour = 0; hour < 24; hour++) {
    const targetTimestamp = startOfDayTimestamp + hour * 3600
    const found = hourlyData.find(item =>
      item.timestamp >= targetTimestamp && item.timestamp < targetTimestamp + 3600,
    )
    if (found) {
      finalHourlyData.push({ ...found, hour })
    }
    else {
      finalHourlyData.push({ hour, uv: 0, pv: 0, timestamp: targetTimestamp })
    }
  }

  const uvData = finalHourlyData.map(item => item.uv)
  const pvData = finalHourlyData.map(item => item.pv)

  return [
    {
      name: 'PV',
      data: pvData,
    },
    {
      name: 'UV',
      data: uvData,
    },
  ]
})
</script>

<template>
  <ACard size="small" :loading="loading">
    <VueApexchart
      v-if="dashboardData"
      :key="`hourly-${theme}`"
      type="bar"
      height="300"
      :options="hourlyChartOptions"
      :series="hourlySeries"
    />
  </ACard>
</template>
