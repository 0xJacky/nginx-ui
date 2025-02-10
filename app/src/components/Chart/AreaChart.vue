<script setup lang="ts">
import type { Series } from '@/components/Chart/types'
import type { Ref } from 'vue'
import { useSettingsStore } from '@/pinia'
import { storeToRefs } from 'pinia'
import VueApexCharts from 'vue3-apexcharts'

const { series, max, yFormatter } = defineProps<{
  series: Series[]
  max?: number
  yFormatter?: (value: number) => string
}>()

const settings = useSettingsStore()
const { theme } = storeToRefs(settings)

function fontColor() {
  return theme.value === 'dark' ? '#b4b4b4' : undefined
}

const chart: Ref<ApexCharts | undefined> = ref()

let chartOptions = {
  chart: {
    type: 'area',
    zoom: {
      enabled: false,
    },
    animations: {
      enabled: false,
    },
    toolbar: {
      show: false,
    },
  },
  colors: ['#ff6385', '#36a3eb'],
  fill: {
    // type: ['solid', 'gradient'],
    gradient: {
      shade: 'light',
    },

    // colors:  ['#ff6385', '#36a3eb'],
  },
  dataLabels: {
    enabled: false,
  },
  stroke: {
    curve: 'smooth',
    width: 0,
  },
  xaxis: {
    type: 'datetime',
    labels: {
      datetimeUTC: false,
      style: {
        colors: fontColor(),
      },
    },
  },
  tooltip: {
    enabled: false,
  },
  yaxis: {
    max,
    tickAmount: 4,
    min: 0,
    labels: {
      style: {
        colors: fontColor(),
      },
      formatter: yFormatter,
    },
  },
  legend: {
    labels: {
      colors: fontColor(),
    },
    onItemClick: {
      toggleDataSeries: false,
    },
    onItemHover: {
      highlightDataSeries: false,
    },
  },
}

function callback() {
  chartOptions = {
    ...chartOptions,
    ...{
      xaxis: {
        type: 'datetime',
        labels: {
          datetimeUTC: false,
          style: {
            colors: fontColor(),
          },
        },
      },
      yaxis: {
        max,
        tickAmount: 4,
        min: 0,
        labels: {
          style: {
            colors: fontColor(),
          },
          formatter: yFormatter,
        },
      },
      legend: {
        labels: {
          colors: fontColor(),
        },
        onItemClick: {
          toggleDataSeries: false,
        },
        onItemHover: {
          highlightDataSeries: false,
        },
      },
    },
  }
  chart.value?.updateOptions?.(chartOptions)
}

watch(theme, callback)
</script>

<template>
  <!-- Use theme as key to rerender the chart when theme changes to prevent style issues -->
  <VueApexCharts
    :key="theme"
    ref="chart"
    type="area"
    height="200"
    :options="chartOptions"
    :series="series"
  />
</template>

<style scoped>

</style>
