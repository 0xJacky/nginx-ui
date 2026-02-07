<script setup lang="ts">
import type { EChartsOption } from 'echarts'
import type { WorldMapData } from '@/api/nginx_log'
import type { GeoData } from '@/composables/useGeoTranslation'
import { ReloadOutlined } from '@ant-design/icons-vue'
import { MapChart } from 'echarts/charts'
import { LegendComponent, TitleComponent, TooltipComponent, VisualMapComponent } from 'echarts/components'
import { registerMap, use } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import countries from 'i18n-iso-countries'
import en from 'i18n-iso-countries/langs/en.json'
import { storeToRefs } from 'pinia'
import VChart from 'vue-echarts'
import { useGeoTranslation } from '@/composables/useGeoTranslation'
import { useSettingsStore } from '@/pinia'
import world from './world.json'

const props = defineProps<{
  data: WorldMapData[] | null
  loading: boolean
  hideCard?: boolean
}>()

const emit = defineEmits<{
  refresh: []
}>()

// Register English locale
countries.registerLocale(en)

// Register ECharts components
use([MapChart, TitleComponent, TooltipComponent, LegendComponent, VisualMapComponent, CanvasRenderer])

const settings = useSettingsStore()
const { theme } = storeToRefs(settings)
const { formatGeoDisplay, translateCountry } = useGeoTranslation()

const chartRef = useTemplateRef<InstanceType<typeof VChart>>('chartRef')

// Register world map on component mount
onMounted(() => {
  registerMap('world', world as unknown as Parameters<typeof registerMap>[1])
})

const fontColor = computed(() => {
  return theme.value === 'dark' ? '#b4b4b4' : '#333'
})

const backgroundColor = computed(() => {
  return theme.value === 'dark' ? 'transparent' : '#fff'
})

// Color scheme for visualMap - darker colors for dark mode to maintain contrast
const visualMapColors = computed(() => {
  return theme.value === 'dark'
    ? ['#1a3a5c', '#1890ff', '#69c0ff'] // Dark mode: darker base, brighter highlights
    : ['#e6f3ff', '#1890ff', '#0050b3'] // Light mode: original colors
})

// Default area color for regions without data
const areaColor = computed(() => {
  return theme.value === 'dark' ? '#2a2a2a' : '#f5f5f5'
})

const mapOption = computed((): EChartsOption => {
  if (!props.data) {
    return {}
  }

  const maxValue = Math.max(...props.data.map(item => item.value))

  // Convert data for ECharts - must use English names to match map data
  const chartData = props.data.map(item => {
    // Always use English name for ECharts map matching
    const englishName = countries.getName(item.code, 'en', { select: 'alias' }) || item.code

    return {
      name: englishName, // Must be English to match world.json
      value: item.value,
      code: item.code,
      localizedName: translateCountry(item.code), // Localized name for tooltip
      rawData: item,
    }
  })

  return {
    backgroundColor: backgroundColor.value,
    tooltip: {
      trigger: 'item',
      formatter: params => {
        const data = params.data as { rawData?: GeoData, name?: string, value?: number, code?: string, localizedName?: string }
        if (data && data.rawData) {
          // Use localized name in tooltip
          const displayName = data.localizedName || data.name || ''
          return `
            <div style="font-size: 14px;">
              <strong>${displayName}</strong><br/>
              ${$gettext('Visits')}: ${data.value || 0}<br/>
              ${$gettext('Percentage')}: ${(data.rawData.percent || 0).toFixed(2)}%
            </div>
          `
        }
        // For countries without data, translate the name
        const englishName = params.name
        const countryCode = countries.getAlpha2Code(englishName, 'en')
        const localizedName = countryCode ? translateCountry(countryCode) : englishName
        return `${localizedName}: ${$gettext('No data')}`
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
        color: visualMapColors.value,
      },
      calculable: false,
    },
    series: [
      {
        name: $gettext('Visits'),
        type: 'map',
        map: 'world',
        roam: false,
        emphasis: {
          label: {
            show: true,
            color: fontColor.value,
            fontSize: 12,
            // eslint-disable-next-line ts/no-explicit-any
            formatter: (params: any) => {
              const data = params.data
              if (data && data.localizedName) {
                return data.localizedName
              }
              // For countries not in top 10, try to get country code from English name
              const englishName = params.name
              // Convert English name back to country code
              const countryCode = countries.getAlpha2Code(englishName, 'en')
              return countryCode ? translateCountry(countryCode) : englishName
            },
          },
          itemStyle: {
            areaColor: theme.value === 'dark' ? '#3a5a7c' : '#ffd666',
          },
        },
        data: chartData,
        itemStyle: {
          areaColor: areaColor.value,
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

// Table data for top 10 countries
const tableData = computed(() => {
  if (!props.data || props.data.length === 0)
    return []

  return props.data.slice(0, 10).map((item, index) => ({
    key: item.code,
    rank: index + 1,
    country: formatGeoDisplay(item),
    code: item.code,
    value: item.value,
    percent: item.percent?.toFixed(2) || '0',
  }))
})

// Table columns
const columns = computed(() => {
  return [
    {
      title: $gettext('Country / Region'),
      dataIndex: 'country',
      key: 'country',
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
      customRender: ({ text }) => `${text}%`,
    },
  ]
})
</script>

<template>
  <ACard v-if="!hideCard" :loading="loading" class="world-map-card">
    <template #title>
      <div class="flex items-center justify-between">
        <span>{{ $gettext('Global Access Map') }}</span>
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
      <AEmpty :description="$gettext('No geographic data available')" />
    </div>

    <div v-else class="world-map-container">
      <!-- Data layout: side by side on large screens, stacked on small screens -->
      <div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <!-- Map on left (or top on small screens) -->
        <div class="lg:col-span-1">
          <VChart
            ref="chartRef"
            :option="mapOption"
            style="height: 400px; width: 100%"
            autoresize
          />
        </div>

        <!-- Table on right (or bottom on small screens) -->
        <div class="lg:col-span-1 flex flex-col justify-center">
          <div class="table-title">
            {{ $gettext('Top 10 Countries / Regions') }}
          </div>
          <ATable
            :columns="columns"
            :data-source="tableData"
            :pagination="false"
            size="small"
            :scroll="{ y: 340 }"
          />
        </div>
      </div>
    </div>
  </ACard>

  <!-- Content without card wrapper when hideCard is true -->
  <div v-else class="world-map-content">
    <div v-if="!data || data.length === 0" class="no-data">
      <AEmpty :description="$gettext('No geographic data available')" />
    </div>

    <div v-else class="world-map-container">
      <!-- Data layout: side by side on large screens, stacked on small screens -->
      <div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <!-- Map on left (or top on small screens) -->
        <div class="lg:col-span-1">
          <VChart
            ref="chartRef"
            :option="mapOption"
            style="height: 400px; width: 100%"
            autoresize
          />
        </div>

        <!-- Table on right (or bottom on small screens) -->
        <div class="lg:col-span-1 flex flex-col justify-center">
          <div class="table-title">
            {{ $gettext('Top 10 Countries / Regions') }}
          </div>
          <ATable
            :columns="columns"
            :data-source="tableData"
            :pagination="false"
            size="small"
            :scroll="{ y: 340 }"
          />
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.world-map-card {
  margin-bottom: 24px;
}

.no-data {
  display: flex;
  justify-content: center;
  align-items: center;
  height: 300px;
}

.table-title {
  margin-bottom: 12px;
  font-size: 14px;
  font-weight: 700;
  color: var(--ant-color-text);
}
</style>
