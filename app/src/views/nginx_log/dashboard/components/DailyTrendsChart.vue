<script setup lang="ts">
import type { DashboardAnalytics } from '@/api/nginx_log'
import { storeToRefs } from 'pinia'
import VueApexchart from 'vue3-apexcharts'
import { useSettingsStore } from '@/pinia'

const props = defineProps<{
  dashboardData: DashboardAnalytics | null
  loading: boolean
}>()

const settings = useSettingsStore()
const { theme } = storeToRefs(settings)

function fontColor() {
  return theme.value === 'dark' ? '#b4b4b4' : undefined
}

const dailyChartOptions = computed(() => {
  if (!props.dashboardData || !props.dashboardData.daily_stats)
    return {}

  const dailyData = props.dashboardData.daily_stats || []
  const dates = dailyData.map(item => item.date)

  return {
    chart: {
      type: 'area',
      height: 300,
      toolbar: {
        show: false,
      },
      zoom: {
        enabled: false,
      },
    },
    title: {
      text: $gettext('Daily Access Trends'),
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
      curve: 'smooth',
      width: 2, // 保持线条显示
    },
    fill: {
      type: 'gradient',
      gradient: {
        shade: 'light',
        type: 'vertical',
        shadeIntensity: 0.5,
        inverseColors: false,
        opacityFrom: 0.8,
        opacityTo: 0.2,
        stops: [0, 100],
      },
    },
    xaxis: {
      categories: dates,
      title: {
        text: $gettext('Date'),
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
    tooltip: {
      shared: true,
      intersect: false,
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

const dailySeries = computed(() => {
  if (!props.dashboardData || !props.dashboardData.daily_stats)
    return []

  const dailyData = props.dashboardData.daily_stats || []
  const uvData = dailyData.map(item => item.uv)
  const pvData = dailyData.map(item => item.pv)

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
      :key="`daily-${theme}`"
      type="area"
      height="300"
      :options="dailyChartOptions"
      :series="dailySeries"
    />
  </ACard>
</template>
