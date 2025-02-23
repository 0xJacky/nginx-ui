<script setup lang="ts">
import type { Series } from '@/components/Chart/types'

import { useSettingsStore } from '@/pinia'
import { storeToRefs } from 'pinia'
import VueApexCharts from 'vue3-apexcharts'

const props = defineProps<{
  series: Series[] | number[]
  centerText?: string
  colors?: string
  name?: string
  bottomText?: string
}>()

const settings = useSettingsStore()

const { theme } = storeToRefs(settings)

function fontColor() {
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
    <p class="bottom-text">
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
      top: -30px;
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

  .bottom-text {
    position: absolute;
    top: calc(106px);
    left: 50%;
    transform: translateX(-50%);
    font-weight: 600;
    width: 100%;
    text-align: center;
  }
}
</style>
