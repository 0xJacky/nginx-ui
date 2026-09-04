<script setup lang="ts">
import type { Series } from '@/components/Chart/types'

import { storeToRefs } from 'pinia'
import VueApexCharts from 'vue3-apexcharts'
import { useSettingsStore } from '@/pinia'

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
    <VueApexCharts
      v-if="centerText"
      class="radialBar"
      type="radialBar"
      height="180"
      :options="chartOptions"
      :series="series"
    />
    <p class="bottom-text">
      {{ bottomText }}
    </p>
  </div>
</template>

<style lang="less" scoped>
.radial-bar-container {
  display: flex;
  flex-direction: column;
  align-items: center;
  margin: 0 auto;

  .radialBar {
    // ApexCharts reserves ~36px of empty canvas above the ring and ~34px below
    // it. Crop that dead space so the card stays compact without shrinking the
    // ring itself.
    margin-top: -30px;
    margin-bottom: -18px;
  }

  .bottom-text {
    margin: 0;
    font-weight: 600;
    text-align: center;
  }
}
</style>
