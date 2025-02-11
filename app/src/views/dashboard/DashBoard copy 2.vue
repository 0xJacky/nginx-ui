<script setup lang="ts">
import { ref, computed } from 'vue'
import { use } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import { PieChart, BarChart } from 'echarts/charts'
import { LineChart } from 'echarts/charts'
import { MapChart } from 'echarts/charts'
import {
  LegendComponent,
  TooltipComponent,
  GridComponent,
  TimelineComponent,
  VisualMapComponent
} from 'echarts/components'
import VChart from 'vue-echarts'
import * as echarts from 'echarts/core'
import 'echarts/map/js/world'

use([
  CanvasRenderer,
  PieChart,
  BarChart,
  LegendComponent,
  TooltipComponent,
  GridComponent,
  LineChart,
  TimelineComponent,
  MapChart,
  VisualMapComponent
])
import {
  BarChartOutlined,
  ClockCircleOutlined,
  SettingOutlined,
  LinkOutlined,
  UserOutlined,
  GlobalOutlined,
  StopOutlined,
  ApiOutlined,
  WarningOutlined,
  CloseCircleOutlined
} from '@ant-design/icons-vue'
import logo from '@/assets/img/logo-primadigi.png'
import background from '@/assets/img/login.mp4'

// Country-specific data
const geoData = ref({
  China: {
    requests: 183200,
    blocked: 45800,
    details: [
      { type: 'Web Attack', count: 12500 },
      { type: 'SQL Injection', count: 8700 },
      { type: 'XSS', count: 6300 }
    ]
  },
  Indonesia: {
    requests: 14800,
    blocked: 3700,
    details: [
      { type: 'Web Attack', count: 1200 },
      { type: 'SQL Injection', count: 850 },
      { type: 'XSS', count: 650 }
    ]
  },
  'United States': {
    requests: 11200,
    blocked: 2800,
    details: [
      { type: 'Web Attack', count: 950 },
      { type: 'SQL Injection', count: 620 },
      { type: 'XSS', count: 430 }
    ]
  }
})

const selectedCountryDetails = ref(null)

const geoChartOption = ref({
  title: {
    text: 'Global Threat Landscape',
    subtext: 'Requests and Blocked Threats',
    left: 'center'
  },
  tooltip: {
    trigger: 'item',
    formatter: (params) => {
      const country = params.name
      const countryData = geoData.value[country]

      if (countryData) {
        return `
          <b>${country}</b><br/>
          Total Requests: ${countryData.requests.toLocaleString()}<br/>
          Blocked Threats: ${countryData.blocked.toLocaleString()}<br/>
          Blocked Rate: ${((countryData.blocked / countryData.requests) * 100).toFixed(2)}%
        `
      }
      return params.name
    }
  },
  visualMap: {
    type: 'continuous',
    min: 0,
    max: 200000,
    text: ['High', 'Low'],
    realtime: false,
    calculable: true,
    inRange: {
      color: ['#lightblue', '#00E7C3', '#3B82F6', '#EF4444']
    }
  },
  series: [
    {
      name: 'Global Threats',
      type: 'map',
      map: 'world',
      roam: true,
      emphasis: {
        label: {
          show: true
        }
      },
      data: Object.keys(geoData.value).map(country => ({
        name: country,
        value: geoData.value[country].blocked
      }))
    }
  ]
})

const handleCountryClick = (params) => {
  const country = params.name
  const countryData = geoData.value[country]

  if (countryData) {
    selectedCountryDetails.value = {
      country: country,
      ...countryData
    }
  }
}


const statistics = ref({
  requests: '198.4k',
  visitors: '114.4k',
  requestIp: '433',
  blocked: '138.4k',
  ipAddr: '22',
  errors4xx: '89.13%',
  errors5xx: '0%'
})

const clientsChartOption = ref({
  tooltip: {
    trigger: 'item',
    formatter: '{a} <br/>{b}: {c} ({d}%)'
  },
  legend: false,
  // legend: {
  //   top: '5%',
  //   left: 'center'
  // },
  series: [
    {
      name: 'Operating Systems',
      type: 'pie',
      radius: ['40%', '70%'],
      center: ['50%', '50%'],
      itemStyle: {
        borderRadius: 10,
        borderColor: '#fff',
        borderWidth: 2
      },
      label: {
        show: true
      },
      data: [
        { value: 1900, name: 'macOS', itemStyle: { color: '#00E7C3' } },
        { value: 869, name: 'Windows', itemStyle: { color: '#3B82F6' } },
        { value: 325, name: 'Linux', itemStyle: { color: '#EF4444' } },
        { value: 150, name: 'Android', itemStyle: { color: '#10B981' } },
        { value: 40, name: 'iOS', itemStyle: { color: '#6366F1' } }
      ]
    },
    {
      name: 'Browsers',
      type: 'pie',
      radius: ['0%', '30%'],
      center: ['50%', '50%'],
      itemStyle: {
        borderRadius: 5,
        borderColor: '#fff',
        borderWidth: 2
      },
      label: {
        show: true
      },
      data: [
        { value: 1500, name: 'Chrome', itemStyle: { color: '#06b6d4' } },
        { value: 274, name: 'IE', itemStyle: { color: '#8b5cf6' } },
        { value: 172, name: 'Headless', itemStyle: { color: '#f59e0b' } },
        { value: 107, name: 'X11', itemStyle: { color: '#ec4899' } }
      ]
    }
  ]
})
const responseStatus = ref([
  { name: '200 OK', value: '120k', color: '#00E7C3' },
  { name: '301 Moved', value: '20k', color: '#3B82F6' },
  { name: '302 Found', value: '15k', color: '#EF4444' },
  { name: '400 Bad Request', value: '80k', color: '#10B981' },
  { name: '403 Forbidden', value: '70k', color: '#6366F1' },
  { name: '404 Not Found', value: '110k', color: '#06b6d4' },
  { name: '500 Server Error', value: '30k', color: '#8b5cf6' },
  { name: '502 Bad Gateway', value: '5k', color: '#f59e0b' },
  { name: '503 Service Unavailable', value: '2k', color: '#ec4899' }
])

const responseChartOption = ref({
  tooltip: {
    trigger: 'item',
    formatter: '{a} <br/>{b}: {c} ({d}%)'
  },
  legend: false,
  series: [
    {
      name: 'Response Status',
      type: 'pie',
      radius: ['40%', '70%'],
      center: ['50%', '50%'],
      itemStyle: {
        borderRadius: 10,
        borderColor: '#fff',
        borderWidth: 2
      },
      label: {
        show: true
      },
      data: [
        { value: 120000, name: '200 OK', itemStyle: { color: '#00E7C3' } },
        { value: 20000, name: '301 Moved', itemStyle: { color: '#3B82F6' } },
        { value: 15000, name: '302 Found', itemStyle: { color: '#EF4444' } },
        { value: 80000, name: '400 Bad Request', itemStyle: { color: '#10B981' } },
        { value: 70000, name: '403 Forbidden', itemStyle: { color: '#6366F1' } },
        { value: 110000, name: '404 Not Found', itemStyle: { color: '#06b6d4' } },
        { value: 30000, name: '500 Server Error', itemStyle: { color: '#8b5cf6' } },
        { value: 5000, name: '502 Bad Gateway', itemStyle: { color: '#f59e0b' } },
        { value: 2000, name: '503 Service Unavailable', itemStyle: { color: '#ec4899' } }
      ]
    }
  ]
})
const leftColumnStatus = computed(() => responseStatus.value.slice(0, Math.ceil(responseStatus.value.length / 2)))
const rightColumnStatus = computed(() => responseStatus.value.slice(Math.ceil(responseStatus.value.length / 2)))

const countryStats = ref([
  { country: 'China', requests: '183.2k' },
  { country: 'Indonesia', requests: '14.8k' },
  { country: 'United States', requests: '112' },
  { country: 'Netherlands', requests: '93' },
  { country: 'United Kingdom', requests: '45' },
  { country: 'India', requests: '28' },
  { country: 'Singapore', requests: '12' }
])

const userClients = ref({
  macOS: { value: '1900', color: '#00E7C3' },
  Windows: { value: '869', color: '#3B82F6' },
  Linux: { value: '325', color: '#EF4444' },
  Android: { value: '150', color: '#10B981' },
  iOS: { value: '40', color: '#6366F1' }
})

const browsers = ref({
  Chrome: { value: '1500', color: '#00E7C3' },
  'Internet Explorer': { value: '274', color: '#3B82F6' },
  'Headless Chrome': { value: '172', color: '#EF4444' },
  'X11': { value: '107', color: '#10B981' }
})

// Generate last 24 hours time labels
const generateHourLabels = () => {
  const labels = []
  const now = new Date()
  for (let i = 23; i >= 0; i--) {
    const hour = new Date(now.getTime() - i * 60 * 60 * 1000)
    labels.push(hour.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }))
  }
  return labels
}

// Mock data for requests per hour (you'll replace with actual API data)
const requestsPerHourData = ref({
  total: [
    120, 95, 110, 88, 105, 130, 145,
    160, 175, 200, 220, 240,
    260, 280, 300, 280, 260,
    240, 220, 200, 180, 160,
    140, 120
  ],
  international: [
    50, 40, 45, 35, 42, 55, 60,
    70, 75, 90, 100, 110,
    120, 130, 140, 130, 120,
    110, 100, 90, 80, 70,
    60, 50
  ]
})

// Mock data for blocking status per hour
const blockingStatusData = ref({
  blocked: [
    10, 8, 12, 6, 9, 15, 18,
    22, 25, 30, 35, 40,
    45, 50, 55, 48, 42,
    38, 32, 28, 24, 20,
    16, 12
  ],
  malicious: [
    5, 4, 6, 3, 4, 7, 9,
    11, 12, 15, 18, 20,
    22, 25, 28, 24, 20,
    18, 16, 14, 12, 10,
    8, 6
  ]
})

const requestsChartOption = ref({
  tooltip: {
    trigger: 'axis',
    axisPointer: {
      type: 'shadow'
    }
  },
  grid: {
    left: '3%',
    right: '4%',
    bottom: '3%',
    containLabel: true
  },
  xAxis: {
    type: 'category',
    data: generateHourLabels()
  },
  yAxis: {
    type: 'value'
  },
  series: [
    {
      name: 'Total Requests',
      type: 'line',
      data: requestsPerHourData.value.total,
      itemStyle: { color: '#00E7C3' }
    },
    {
      name: 'International Requests',
      type: 'line',
      data: requestsPerHourData.value.international,
      itemStyle: { color: '#3B82F6' }
    }
  ]
})

const blockingChartOption = ref({
  tooltip: {
    trigger: 'axis',
    axisPointer: {
      type: 'shadow'
    }
  },
  grid: {
    left: '3%',
    right: '4%',
    bottom: '3%',
    containLabel: true
  },
  xAxis: {
    type: 'category',
    data: generateHourLabels()
  },
  yAxis: {
    type: 'value'
  },
  series: [
    {
      name: 'Blocked Requests',
      type: 'line',
      data: blockingStatusData.value.blocked,
      itemStyle: { color: '#EF4444' }
    },
    {
      name: 'Malicious Requests',
      type: 'line',
      data: blockingStatusData.value.malicious,
      itemStyle: { color: '#10B981' }
    }
  ]
})

const thisYear = new Date().getFullYear()
</script>

<template>
  <div>
    <!-- Statistics Bar -->
    <div class="grid grid-cols-7 gap-4 mb-8">
      <div class="bg-white rounded-lg shadow p-4">
        <div class="flex items-center gap-2">
          <LinkOutlined class="text-[#00E7C3]" />
          <div class="text-sm text-gray-600">Requests</div>
        </div>
        <div class="text-2xl font-bold">{{ statistics.requests }}</div>
      </div>
      <div class="bg-white rounded-lg shadow p-4">
        <div class="flex items-center gap-2">
          <UserOutlined class="text-[#00E7C3]" />
          <div class="text-sm text-gray-600">Visitors</div>
        </div>
        <div class="text-2xl font-bold">{{ statistics.visitors }}</div>
      </div>
      <div class="bg-white rounded-lg shadow p-4">
        <div class="flex items-center gap-2">
          <GlobalOutlined class="text-[#00E7C3]" />
          <div class="text-sm text-gray-600">Request IP</div>
        </div>
        <div class="text-2xl font-bold">{{ statistics.requestIp }}</div>
      </div>
      <div class="bg-white rounded-lg shadow p-4">
        <div class="flex items-center gap-2">
          <StopOutlined class="text-[#00E7C3]" />
          <div class="text-sm text-gray-600">Blocked</div>
        </div>
        <div class="text-2xl font-bold">{{ statistics.blocked }}</div>
      </div>
      <div class="bg-white rounded-lg shadow p-4">
        <div class="flex items-center gap-2">
          <ApiOutlined class="text-[#00E7C3]" />
          <div class="text-sm text-gray-600">IP Addr</div>
        </div>
        <div class="text-2xl font-bold">{{ statistics.ipAddr }}</div>
      </div>
      <div class="bg-white rounded-lg shadow p-4">
        <div class="flex items-center gap-2">
          <WarningOutlined class="text-[#00E7C3]" />
          <div class="text-sm text-gray-600">4xx Errors</div>
        </div>
        <div class="text-2xl font-bold text-red-500">{{ statistics.errors4xx }}</div>
      </div>
      <div class="bg-white rounded-lg shadow p-4">
        <div class="flex items-center gap-2">
          <CloseCircleOutlined class="text-[#00E7C3]" />
          <div class="text-sm text-gray-600">5xx Errors</div>
        </div>
        <div class="text-2xl font-bold">{{ statistics.errors5xx }}</div>
      </div>
    </div>
    <!-- Main Dashboard Grid -->
    <div class="grid grid-cols-3 gap-6">
      <!-- Globe Section -->
      <!-- Globe Section -->
      <div class="col-span-2 bg-white rounded-lg shadow p-6">
        <div class="flex justify-between items-center mb-4">
          <h2 class="text-lg font-semibold">Geo Location Threat Map</h2>
          <div class="flex space-x-2">
            <span class="bg-[#00E7C3] text-white px-2 py-1 rounded text-sm">Interactive</span>
          </div>
        </div>
        <v-chart class="geo-chart" :option="geoChartOption" @click="handleCountryClick" autoresize />
      </div>

      <!-- Country Details Section -->
      <div class="bg-white rounded-lg shadow p-6">
        <h2 class="text-lg font-semibold mb-4">Country Details</h2>
        <div v-if="selectedCountryDetails" class="space-y-4">
          <h3 class="text-xl font-bold">{{ selectedCountryDetails.country }}</h3>
          <div class="grid grid-cols-2 gap-2">
            <div>
              <span class="text-sm text-gray-600">Total Requests</span>
              <div class="text-lg font-semibold">
                {{ selectedCountryDetails.requests.toLocaleString() }}
              </div>
            </div>
            <div>
              <span class="text-sm text-gray-600">Blocked Threats</span>
              <div class="text-lg font-semibold text-red-500">
                {{ selectedCountryDetails.blocked.toLocaleString() }}
              </div>
            </div>
          </div>
          <div>
            <h4 class="text-md font-semibold mb-2">Threat Breakdown</h4>
            <div v-for="detail in selectedCountryDetails.details" :key="detail.type"
              class="flex justify-between border-b py-1">
              <span>{{ detail.type }}</span>
              <span class="font-semibold">{{ detail.count }}</span>
            </div>
          </div>
        </div>
        <div v-else class="text-center text-gray-500">
          Click a country to view details
        </div>
      </div>

      <!-- Country Stats -->
      <div class="grid grid-cols-1 gap-6">
        <!-- Requests Status -->
        <div class="bg-white rounded-lg shadow p-6">
          <div class="flex justify-between items-center mb-4">
            <h2 class="text-lg font-semibold">Requests Status (Last 24 Hours)</h2>
          </div>
          <v-chart class="request-chart" :option="requestsChartOption" autoresize />
        </div>

        <!-- Blocking Status -->
        <div class="bg-white rounded-lg shadow p-6">
          <div class="flex justify-between items-center mb-4">
            <h2 class="text-lg font-semibold">Blocking Status (Last 24 Hours)</h2>
          </div>
          <v-chart class="request-chart" :option="blockingChartOption" autoresize />
        </div>
      </div>
    </div>

    <!-- Bottom Section -->
    <div class="grid grid-cols-2 gap-6 mt-6">
      <!-- User Clients & Browsers Section -->
      <div class="bg-white rounded-lg shadow p-6">
        <div class="flex justify-between items-center mb-4">
          <h2 class="text-lg font-semibold">User Clients</h2>
        </div>
        <div class="flex">
          <!-- Chart on the left -->
          <div class="w-1/2">
            <v-chart class="small-chart" :option="clientsChartOption" autoresize />
          </div>

          <!-- Stats cards on the right -->
          <div class="w-1/2 bg-gray-100 rounded-lg p-4">
            <div class="flex">
              <!-- OS Stats -->
              <div class="w-1/2 space-y-3">
                <h3 class="font-semibold mb-4">Operating Systems</h3>
                <div v-for="(client, name) in userClients" :key="name" class="flex flex-col">
                  <div class="flex items-center gap-2">
                    <div class="w-3 h-3 rounded-full" :style="{ backgroundColor: client.color }"></div>
                    <span class="text-sm">{{ name }}</span>
                  </div>
                  <span class="text-sm font-semibold ml-5">{{ client.value }}</span>
                </div>
              </div>

              <!-- Browser Stats -->
              <div class="w-1/2 space-y-3">
                <h3 class="font-semibold mb-4">Browsers</h3>
                <div v-for="(browser, name) in browsers" :key="name" class="flex flex-col">
                  <div class="flex items-center gap-2">
                    <div class="w-3 h-3 rounded-full" :style="{ backgroundColor: browser.color }"></div>
                    <span class="text-sm">{{ name }}</span>
                  </div>
                  <span class="text-sm font-semibold ml-5">{{ browser.value }}</span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Response Status -->
      <div class="bg-white rounded-lg shadow p-6">
        <div class="flex justify-between items-center mb-4">
          <h2 class="text-lg font-semibold">Response Status</h2>
        </div>
        <div class="flex">
          <!-- Chart on the left -->
          <div class="w-1/2">
            <v-chart class="mini-chart" :option="responseChartOption" autoresize />
          </div>

          <!-- Stats on the right -->
          <div class="w-1/2 bg-gray-100 rounded-lg p-4">
            <div class="flex">
              <!-- Left Column -->
              <div class="w-1/2 space-y-2">
                <div v-for="status in leftColumnStatus" :key="status.name" class="flex flex-col">
                  <div class="flex items-center gap-2">
                    <div class="w-2 h-2 rounded-full" :style="{ backgroundColor: status.color }"></div>
                    <span class="text-xs">{{ status.name }}</span>
                  </div>
                  <span class="text-xs font-semibold ml-4">{{ status.value }}</span>
                </div>
              </div>

              <!-- Right Column -->
              <div class="w-1/2 space-y-2">
                <div v-for="status in rightColumnStatus" :key="status.name" class="flex flex-col">
                  <div class="flex items-center gap-2">
                    <div class="w-2 h-2 rounded-full" :style="{ backgroundColor: status.color }"></div>
                    <span class="text-xs">{{ status.name }}</span>
                  </div>
                  <span class="text-xs font-semibold ml-4">{{ status.value }}</span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

    </div>
  </div>
</template>

<style scoped>
.geo-chart {
  height: 500px;
  width: 100%;
}

.request-chart {
  height: 200px;
}

.mini-chart {
  height: 300px;
}

.chart {
  height: 400px;
}

.shadow {
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.05);
}

:deep(.ant-btn-primary) {
  background-color: #00E7C3;
  border-color: #00E7C3;
}

:deep(.ant-btn-primary:hover) {
  background-color: #00d1b0;
  border-color: #00d1b0;
}

.dark {
  .ant-layout-content {
    background: transparent;
  }

  .bg-white {
    background-color: #1f1f1f;
  }

  .text-gray-700 {
    color: #e5e5e5;
  }
}
</style>
