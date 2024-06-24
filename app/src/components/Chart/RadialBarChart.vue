<script setup lang="ts">
import VueApexCharts from 'vue3-apexcharts'

import { storeToRefs } from 'pinia'
import { useSettingsStore } from '@/pinia'
import type { Series } from '@/components/Chart/types'

const props = defineProps<{
  series: Series[] | number[]
  centerText?: string
  colors?: string
  name?: string
  bottomText?: string
}>()

const settings = useSettingsStore()

const { theme } = storeToRefs(settings)

const fontColor = () => {
  return theme.value === 'dark' ? '#fcfcfc' : undefined
}

const chartOptions = computed(() => ({
  series: props.series,
  chart: {
    type: 'radialBar',
    offsetY: 0,
  },
  plotOptions: {
    radialBar: {
      startAngle: -135,
      endAngle: 135,
      dataLabels: {
        name: {
          fontSize: '14px',
          color: props.colors,
          offsetY: 36,
        },
        value: {
          offsetY: -12,
          fontSize: '14px',
          color: fontColor(),
          formatter: () => {
            return props.centerText
          },
        },
      },
    },
  },
  fill: {
    colors: props.colors,
  },
  labels: [props.name],
  states: {
    hover: {
      filter: {
        type: 'none',
      },
    },
    active: {
      filter: {
        type: 'none',
      },
    },
  },
}))
</script>

<template>
  <!-- Use theme as key to rerender the chart when theme changes to prevent style issues -->
  <div
    :key="theme"
    class="radial-bar-container"
  >
    <p class="bottom_text">
      {{ bottomText }}
    </p>
    <VueApexCharts
      v-if="centerText"
      class="radialBar"
      type="radialBar"
      height="205"
      :options="chartOptions"
      :series="series"
    />
  </div>
</template>

<style lang="less" scoped>
.radial-bar-container {
  position: relative;
  margin: 0 auto;
  height: 112px !important;

  .radialBar {
    position: absolute;
    top: -30px;
    @media (max-width: 1700px) and (min-width: 1200px) {
      top: -10px;
    }
    @media (max-width: 768px) and (min-width: 290px) {
      left: 50%;
      transform: translateX(-50%);
    }
  }

  .text {
    position: absolute;
    width: 100%;
    text-align: center;
  }

  .bottom_text {
    position: absolute;
    top: calc(106px);
    @media (max-width: 1300px) and (min-width: 1200px) {
      top: calc(96px);
    }
    font-weight: 600;
    width: 100%;
    text-align: center;
  }
}
</style>
