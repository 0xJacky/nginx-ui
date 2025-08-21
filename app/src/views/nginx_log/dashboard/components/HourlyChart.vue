<script setup lang="ts">
import type { DashboardAnalytics } from '@/api/nginx_log'
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

  const hourlyData = props.dashboardData.hourly_stats || []
  const hours = hourlyData.map(item => `${item.hour}`)

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

  const hourlyData = props.dashboardData.hourly_stats || []
  const uvData = hourlyData.map(item => item.uv)
  const pvData = hourlyData.map(item => item.pv)

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
