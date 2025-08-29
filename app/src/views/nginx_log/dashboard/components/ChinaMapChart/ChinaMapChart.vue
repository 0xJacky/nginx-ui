<script setup lang="ts">
import type { EChartsOption } from 'echarts'
import { ReloadOutlined } from '@ant-design/icons-vue'
import { MapChart } from 'echarts/charts'
import { LegendComponent, TitleComponent, TooltipComponent, VisualMapComponent } from 'echarts/components'
import { registerMap, use } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import { storeToRefs } from 'pinia'
import VChart from 'vue-echarts'
import { useSettingsStore } from '@/pinia'
import china from './china.json'

const props = defineProps<{
  data: ChinaMapData[] | null
  loading: boolean
  hideCard?: boolean
}>()

const emit = defineEmits<{
  refresh: []
}>()

// Register ECharts components
use([MapChart, TitleComponent, TooltipComponent, LegendComponent, VisualMapComponent, CanvasRenderer])

interface CityData {
  name: string
  value: number
  percent: number
}

interface ChinaMapData {
  name: string
  value: number
  percent: number
  cities?: CityData[]
}

const settings = useSettingsStore()
const { theme } = storeToRefs(settings)

// Table data for top 10 provinces
const tableData = computed(() => {
  if (!props.data || props.data.length === 0)
    return []

  return props.data.slice(0, 10).map(item => ({
    key: item.name,
    province: item.name,
    value: item.value,
    percent: item.percent?.toFixed(2) || '0.00',
  }))
})

// Table columns
const columns = computed(() => {
  return [
    {
      title: $gettext('Province / Region'),
      dataIndex: 'province',
      key: 'province',
    },
    {
      title: $gettext('Visits'),
      dataIndex: 'value',
      key: 'value',
      align: 'right' as const,
      sorter: (a: Record<string, unknown>, b: Record<string, unknown>) => (a.value as number) - (b.value as number),
      customRender: ({ text }) => `${text.toLocaleString()}`,
    },
    {
      title: $gettext('Percentage'),
      dataIndex: 'percent',
      key: 'percent',
      align: 'right' as const,
      sorter: (a: Record<string, unknown>, b: Record<string, unknown>) => Number.parseFloat(a.percent as string) - Number.parseFloat(b.percent as string),
      customRender: ({ text }: { text: string }) => `${text}%`,
    },
  ]
})

const chartRef = useTemplateRef<InstanceType<typeof VChart>>('chartRef')

// Register China map on component mount
onMounted(() => {
  registerMap('china', china as unknown as Parameters<typeof registerMap>[1])
})

const fontColor = computed(() => {
  return theme.value === 'dark' ? '#b4b4b4' : '#333'
})

const backgroundColor = computed(() => {
  return theme.value === 'dark' ? 'transparent' : '#fff'
})

const mapOption = computed((): EChartsOption => {
  if (!props.data) {
    return {}
  }

  const maxValue = Math.max(...props.data.map(item => item.value))

  // Convert data for ECharts map
  const chartData = props.data.map(item => ({
    name: item.name,
    value: item.value,
  }))

  return {
    backgroundColor: backgroundColor.value,
    tooltip: {
      trigger: 'item',
      formatter: params => {
        if (params.data) {
          const item = props.data?.find(d => d.name === params.data.name)
          if (item) {
            return `
                <div style="font-size: 14px;">
                  <strong>${item.name}</strong><br/>
                  ${$gettext('Visits')}: ${item.value}<br/>
                  ${$gettext('Percentage')}: ${item.percent.toFixed(2)}%
                </div>
              `
          }
        }
        return `${params.name}: ${$gettext('No data')}`
      },
    },
    visualMap: {
      min: 0,
      max: maxValue,
      left: 'left',
      top: 'bottom',
      text: [$gettext('High'), $gettext('Low')],
      textStyle: {
        color: fontColor.value,
      },
      inRange: {
        color: ['#fff2e8', '#ffbb96', '#ff7a45', '#fa541c', '#d4380d'],
      },
      calculable: false,
    },
    series: [
      {
        name: $gettext('Visits'),
        type: 'map',
        map: 'china',
        roam: false,
        emphasis: {
          label: {
            show: true,
            color: fontColor.value,
          },
          itemStyle: {
            areaColor: '#f7d794',
          },
        },
        data: chartData,
        itemStyle: {
          borderColor: theme.value === 'dark' ? '#555' : '#ddd',
          borderWidth: 0.5,
        },
      },
    ],
  }
})

// Handle theme changes
watch(theme, () => {
  if (chartRef.value) {
    chartRef.value.setOption(mapOption.value, true)
  }
})
</script>

<template>
  <ACard v-if="!hideCard" :loading="loading" class="china-map-card">
    <template #title>
      <div class="flex items-center justify-between">
        <span>{{ $gettext('China Access Map') }}</span>
        <AButton
          type="text"
          size="small"
          :loading="loading"
          @click="emit('refresh')"
        >
          <template #icon>
            <ReloadOutlined />
          </template>
        </AButton>
      </div>
    </template>

    <div v-if="!data || data.length === 0" class="no-data">
      <AEmpty :description="$gettext('No China geographic data available')" />
    </div>

    <div v-else class="china-map-container">
      <!-- Data layout: side by side on large screens, stacked on small screens -->
      <div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <!-- Map on left (or top on small screens) -->
        <div class="lg:col-span-1">
          <VChart
            ref="chartRef"
            :option="mapOption"
            style="height: 500px; width: 100%"
            autoresize
          />
        </div>

        <!-- Table on right (or bottom on small screens) -->
        <div class="lg:col-span-1 flex flex-col justify-center">
          <div class="mb-3 text-sm font-bold text-gray-800">
            {{ $gettext('Top 10 Provinces / Regions') }}
          </div>
          <ATable
            :columns="columns"
            :data-source="tableData"
            :pagination="false"
            size="small"
            :scroll="{ y: 440 }"
          />
        </div>
      </div>
    </div>
  </ACard>

  <!-- Content without card wrapper when hideCard is true -->
  <div v-else class="china-map-content">
    <div v-if="!data || data.length === 0" class="no-data">
      <AEmpty :description="$gettext('No China geographic data available')" />
    </div>

    <div v-else class="china-map-container">
      <!-- Data layout: side by side on large screens, stacked on small screens -->
      <div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <!-- Map on left (or top on small screens) -->
        <div class="lg:col-span-1">
          <VChart
            ref="chartRef"
            :option="mapOption"
            style="height: 500px; width: 100%"
            autoresize
          />
        </div>

        <!-- Table on right (or bottom on small screens) -->
        <div class="lg:col-span-1 flex flex-col justify-center">
          <div class="mb-3 text-sm font-bold text-gray-800">
            {{ $gettext('Top 10 Provinces / Regions') }}
          </div>
          <ATable
            :columns="columns"
            :data-source="tableData"
            :pagination="false"
            size="small"
            :scroll="{ y: 440 }"
          />
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.china-map-card {
  margin-bottom: 24px;
}

.no-data {
  display: flex;
  justify-content: center;
  align-items: center;
  height: 400px;
}

.province-summary {
  padding: 12px;
  background: var(--ant-color-bg-container);
  border: 1px solid var(--ant-color-border);
  border-radius: 6px;
  text-align: center;
}

.province-name {
  font-weight: 600;
  font-size: 14px;
  margin-bottom: 4px;
}

.province-value {
  font-size: 18px;
  font-weight: 700;
  color: var(--ant-color-primary);
  margin-bottom: 2px;
}

.province-percent {
  font-size: 12px;
  color: var(--ant-color-text-secondary);
  margin-bottom: 8px;
}

.cities-list {
  border-top: 1px solid var(--ant-color-border);
  padding-top: 8px;
}

.cities-title {
  font-size: 12px;
  font-weight: 600;
  margin-bottom: 4px;
  color: var(--ant-color-text-secondary);
}

.city-item {
  font-size: 11px;
  color: var(--ant-color-text-tertiary);
  line-height: 1.4;
}
</style>
