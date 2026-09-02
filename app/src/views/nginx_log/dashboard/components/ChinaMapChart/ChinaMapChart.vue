<script setup lang="ts">
import type { EChartsOption } from 'echarts'
import { LeftOutlined, ReloadOutlined } from '@ant-design/icons-vue'
import { MapChart } from 'echarts/charts'
import { LegendComponent, TitleComponent, TooltipComponent, VisualMapComponent } from 'echarts/components'
import { registerMap, use } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import { storeToRefs } from 'pinia'
import VChart from 'vue-echarts'
import nginx_log from '@/api/nginx_log'
import { useSettingsStore } from '@/pinia'
import china from './china.json'

const props = defineProps<{
  data: ChinaMapData[] | null
  loading: boolean
  hideCard?: boolean
  logPath: string
  startTime: number
  endTime: number
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

// Public boundary API for province-level "full" geojson (includes city children).
// Only province adcodes present in china.json are ever requested.
const CITY_BOUND_API = 'https://geo.datav.aliyun.com/areas_v3/bound'

const settings = useSettingsStore()
const { theme } = storeToRefs(settings)

// Drill-down state: null means showing the province-level China map.
interface DrilldownState {
  name: string
  adcode: string
  mapName: string
}
const drilldown = ref<DrilldownState | null>(null)
const drillLoading = ref(false)
const cityChartData = ref<CityData[]>([])

// Cache of already-fetched/registered city-level maps, keyed by adcode.
const registeredCityMaps = new Map<string, string[]>()

function findProvinceAdcode(name: string): string | null {
  const feature = (china as { features: Array<{ id: string, properties: { name: string } }> }).features
    .find(f => f.properties.name === name)
  return feature?.id ?? null
}

// City boundary feature names carry administrative suffixes (市/县/州/盟/地区)
// that our aggregated city data does not, so match by prefix instead of equality.
function matchCityName(rawName: string, featureNames: string[]): string {
  return featureNames.find(f => f === rawName || f.startsWith(rawName)) ?? rawName
}

async function drillIntoProvince(name: string) {
  if (drillLoading.value || drilldown.value?.name === name)
    return

  const adcode = findProvinceAdcode(name)
  if (!adcode)
    return

  drillLoading.value = true
  try {
    const mapName = `china-city-${adcode}`
    let featureNames = registeredCityMaps.get(adcode)

    const [cityStats] = await Promise.all([
      nginx_log.getChinaCityMapData({
        path: props.logPath,
        start_time: props.startTime,
        end_time: props.endTime,
        province: name,
      }),
      (async () => {
        if (featureNames)
          return
        const res = await fetch(`${CITY_BOUND_API}/${adcode}_full.json`)
        if (!res.ok)
          throw new Error(`Failed to fetch city boundaries: ${res.status}`)
        const geojson = await res.json()
        registerMap(mapName, geojson as unknown as Parameters<typeof registerMap>[1])
        featureNames = (geojson.features as Array<{ properties: { name: string } }>).map(f => f.properties.name)
        registeredCityMaps.set(adcode, featureNames)
      })(),
    ])

    cityChartData.value = cityStats.data.map(city => ({
      ...city,
      name: matchCityName(city.name, featureNames!),
    }))
    drilldown.value = { name, adcode, mapName }
  }
  catch {
    message.error($gettext('Failed to load city map data'))
  }
  finally {
    drillLoading.value = false
  }
}

function backToProvinces() {
  drilldown.value = null
  cityChartData.value = []
}

// Reset drill-down whenever fresh data arrives so stale city data is never shown.
watch(() => props.data, () => backToProvinces())

// Table data for top 10 provinces, or top 10 cities once drilled into a province.
const tableData = computed(() => {
  const source = drilldown.value ? cityChartData.value : props.data

  if (!source || source.length === 0)
    return []

  return source.slice(0, 10).map(item => ({
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
      title: drilldown.value ? $gettext('City') : $gettext('Province / Region'),
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

// Color scheme for visualMap - brighter colors for dark mode to maintain visibility
const visualMapColors = computed(() => {
  return theme.value === 'dark'
    ? ['#612500', '#ad4e00', '#d4380d', '#ff7a45', '#ffbb96'] // Dark mode: visible orange gradient
    : ['#fff2e8', '#ffbb96', '#ff7a45', '#fa541c', '#d4380d'] // Light mode: original colors
})

// Default area color for regions without data
const areaColor = computed(() => {
  return theme.value === 'dark' ? '#2a2a2a' : '#f5f5f5'
})

// Tooltip style for dark mode
const tooltipBgColor = computed(() => {
  return theme.value === 'dark' ? 'rgba(50, 50, 50, 0.9)' : 'rgba(255, 255, 255, 0.9)'
})

const tooltipBorderColor = computed(() => {
  return theme.value === 'dark' ? '#555' : '#ccc'
})

const tooltipTextColor = computed(() => {
  return theme.value === 'dark' ? '#e0e0e0' : '#333'
})

const mapOption = computed((): EChartsOption => {
  // Outside drilldown, no province data means nothing to show at all.
  if (!drilldown.value && (!props.data || props.data.length === 0)) {
    return {}
  }

  const source = drilldown.value ? cityChartData.value : props.data!
  const maxValue = source.length > 0 ? Math.max(...source.map(item => item.value)) : 0

  // Convert data for ECharts map
  const chartData = source.map(item => ({
    name: item.name,
    value: item.value,
  }))

  return {
    backgroundColor: backgroundColor.value,
    tooltip: {
      trigger: 'item',
      backgroundColor: tooltipBgColor.value,
      borderColor: tooltipBorderColor.value,
      textStyle: {
        color: tooltipTextColor.value,
      },
      formatter: params => {
        if (params.data) {
          const item = source.find(d => d.name === params.data.name)
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
        color: visualMapColors.value,
      },
      calculable: false,
    },
    series: [
      {
        name: $gettext('Visits'),
        type: 'map',
        map: drilldown.value ? drilldown.value.mapName : 'china',
        roam: false,
        emphasis: {
          label: {
            show: true,
            color: fontColor.value,
          },
          itemStyle: {
            areaColor: theme.value === 'dark' ? '#5c3a2a' : '#f7d794',
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

// vue-echarts merges option updates by default, which leaves the previous
// map's geo component cached and renders a blank chart when the series
// `map` name changes; force a full replace whenever we switch maps.
watch(() => drilldown.value?.mapName, () => {
  if (chartRef.value) {
    chartRef.value.setOption(mapOption.value, true)
  }
})

// Clicking a province drills down into its city-level map; already-drilled
// clicks are ignored since city boundaries have no further children here.
function handleChartClick(params: { name?: string }) {
  if (drilldown.value || !params.name)
    return
  drillIntoProvince(params.name)
}
</script>

<template>
  <ACard v-if="!hideCard" :loading="loading" class="china-map-card">
    <template #title>
      <div class="flex items-center justify-between">
        <span class="flex items-center gap-2">
          <AButton
            v-if="drilldown"
            type="text"
            size="small"
            @click="backToProvinces"
          >
            <template #icon>
              <LeftOutlined />
            </template>
          </AButton>
          <span>{{ drilldown ? drilldown.name : $gettext('China Access Map') }}</span>
        </span>
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
            :loading="drillLoading"
            style="height: 500px; width: 100%"
            autoresize
            @click="handleChartClick"
          />
        </div>

        <!-- Table on right (or bottom on small screens) -->
        <div class="lg:col-span-1 flex flex-col justify-center">
          <div class="table-title">
            {{ drilldown ? $gettext('Top 10 Cities') : $gettext('Top 10 Provinces / Regions') }}
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
      <div v-if="drilldown" class="flex items-center gap-2 mb-3">
        <AButton type="text" size="small" @click="backToProvinces">
          <template #icon>
            <LeftOutlined />
          </template>
        </AButton>
        <span class="font-medium">{{ drilldown.name }}</span>
      </div>

      <!-- Data layout: side by side on large screens, stacked on small screens -->
      <div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <!-- Map on left (or top on small screens) -->
        <div class="lg:col-span-1">
          <VChart
            ref="chartRef"
            :option="mapOption"
            :loading="drillLoading"
            style="height: 500px; width: 100%"
            autoresize
            @click="handleChartClick"
          />
        </div>

        <!-- Table on right (or bottom on small screens) -->
        <div class="lg:col-span-1 flex flex-col justify-center">
          <div class="table-title">
            {{ drilldown ? $gettext('Top 10 Cities') : $gettext('Top 10 Provinces / Regions') }}
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

.table-title {
  margin-bottom: 12px;
  font-size: 14px;
  font-weight: 700;
  color: var(--ant-color-text);
}
</style>
