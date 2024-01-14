<script setup lang="ts">
import { useGettext } from 'vue3-gettext'
import type ReconnectingWebSocket from 'reconnecting-websocket'
import AreaChart from '@/components/Chart/AreaChart.vue'
import RadialBarChart from '@/components/Chart/RadialBarChart.vue'
import type { CPUInfoStat, DiskStat, HostInfoStat, LoadStat, MemStat } from '@/api/analytic'
import analytic from '@/api/analytic'
import { bytesToSize } from '@/lib/helper'
import type { Series } from '@/components/Chart/types'

const { $gettext } = useGettext()

let websocket: ReconnectingWebSocket | WebSocket

const host: HostInfoStat = reactive({
  platform: '',
  platformVersion: '',
  os: '',
  kernelVersion: '',
  kernelArch: '',
}) as HostInfoStat

const cpu = ref('0.0')
const cpu_info = reactive([]) as CPUInfoStat[]
const cpu_analytic_series = reactive([{ name: 'User', data: [] }, { name: 'Total', data: [] }]) as Series[]

const net_analytic = reactive([{ name: $gettext('Receive'), data: [] },
  { name: $gettext('Send'), data: [] }]) as Series[]

const disk_io_analytic = reactive([{ name: $gettext('Writes'), data: [] },
  { name: $gettext('Reads'), data: [] }]) as Series[]

const memory = reactive({
  total: '',
  used: '',
  cached: '',
  free: '',
  swap_used: '',
  swap_cached: '',
  swap_percent: 0,
  swap_total: '',
  pressure: 0,
}) as MemStat

const disk = reactive({ percentage: 0, used: '', total: '', writes: { x: '', y: 0 }, reads: { x: '', y: 0 } }) as DiskStat
const disk_io = reactive({ writes: 0, reads: 0 })
const uptime = ref('')
const loadavg = reactive({ load1: 0, load5: 0, load15: 0 }) as LoadStat
const net = reactive({ recv: 0, sent: 0, last_recv: 0, last_sent: 0 })

const net_formatter = (bytes: number) => {
  return `${bytesToSize(bytes)}/s`
}

const cpu_formatter = (usage: number) => {
  return usage.toFixed(2)
}

onMounted(() => {
  analytic.init().then(r => {
    Object.assign(host, r.host)
    Object.assign(cpu_info, r.cpu.info)
    Object.assign(memory, r.memory)
    Object.assign(disk, r.disk)

    // uptime
    handle_uptime(r.host?.uptime)

    // load_avg
    Object.assign(loadavg, r.loadavg)

    net.last_recv = r.network.init.bytesRecv
    net.last_sent = r.network.init.bytesSent

    cpu_analytic_series[0].data = cpu_analytic_series[0].data.concat(r.cpu.user)
    cpu_analytic_series[1].data = cpu_analytic_series[1].data.concat(r.cpu.total)
    net_analytic[0].data = net_analytic[0].data.concat(r.network.bytesRecv)
    net_analytic[1].data = net_analytic[1].data.concat(r.network.bytesSent)
    disk_io_analytic[0].data = disk_io_analytic[0].data.concat(r.disk_io.writes)
    disk_io_analytic[1].data = disk_io_analytic[1].data.concat(r.disk_io.reads)

    websocket = analytic.server()
    websocket.onmessage = wsOnMessage
  })
})

onUnmounted(() => {
  websocket.close()
})

function handle_uptime(t: number) {
  // uptime
  let _uptime = Math.floor(t)
  const uptime_days = Math.floor(_uptime / 86400)

  _uptime -= uptime_days * 86400

  const uptime_hours = Math.floor(_uptime / 3600)

  _uptime -= uptime_hours * 3600
  uptime.value = `${uptime_days}d ${uptime_hours}h ${Math.floor(_uptime / 60)}m`
}

function wsOnMessage(m: MessageEvent) {
  const r = JSON.parse(m.data)

  const cpu_usage = Math.min(r.cpu.system + r.cpu.user, 100)

  cpu.value = cpu_usage.toFixed(2)

  const time = new Date().toLocaleString()

  cpu_analytic_series[0].data.push({ x: time, y: r.cpu.user.toFixed(2) })
  cpu_analytic_series[1].data.push({ x: time, y: cpu_usage })

  if (cpu_analytic_series[0].data.length > 100) {
    cpu_analytic_series[0].data.shift()
    cpu_analytic_series[1].data.shift()
  }

  // mem
  Object.assign(memory, r.memory)

  // disk
  Object.assign(disk, r.disk)
  disk_io.writes = r.disk.writes.y
  disk_io.reads = r.disk.reads.y

  // uptime
  handle_uptime(r.uptime)

  // loadavg
  Object.assign(loadavg, r.loadavg)

  // network
  Object.assign(net, r.network)
  net.recv = r.network.bytesRecv - net.last_recv
  net.sent = r.network.bytesSent - net.last_sent
  net.last_recv = r.network.bytesRecv
  net.last_sent = r.network.bytesSent

  net_analytic[0].data.push({ x: time, y: net.recv })
  net_analytic[1].data.push({ x: time, y: net.sent })

  if (net_analytic[0].data.length > 100) {
    net_analytic[0].data.shift()
    net_analytic[1].data.shift()
  }

  disk_io_analytic[0].data.push(r.disk.writes)
  disk_io_analytic[1].data.push(r.disk.reads)

  if (disk_io_analytic[0].data.length > 100) {
    disk_io_analytic[0].data.shift()
    disk_io_analytic[1].data.shift()
  }
}
</script>

<template>
  <div>
    <ARow
      :gutter="[{ xs: 0, sm: 16 }, 16]"
      class="first-row"
    >
      <ACol
        :xl="7"
        :lg="24"
        :md="24"
        :xs="24"
      >
        <ACard
          :title="$gettext('Server Info')"
          :bordered="false"
        >
          <p>
            {{ $gettext('Uptime:') }}
            {{ uptime }}
          </p>
          <p>
            {{ $gettext('Load Average:') }}
            <span class="load-avg-describe"> 1min:</span>{{ ` ${loadavg?.load1?.toFixed(2)}` }}
            <span class="load-avg-describe"> | 5min:</span>{{ loadavg?.load5?.toFixed(2) }}
            <span class="load-avg-describe"> | 15min:</span>{{ loadavg?.load15?.toFixed(2) }}
          </p>
          <p>
            {{ $gettext('OS:') }}
            <span class="os-platform">{{ ` ${host.platform}` }}</span> {{ host.platformVersion }}
            <span class="os-info">({{ host.os }} {{ host.kernelVersion }}
              {{ host.kernelArch }})</span>
          </p>
          <p v-if="cpu_info">
            {{ `${$gettext('CPU:')} ` }}
            <span class="cpu-model">{{ cpu_info[0]?.modelName || 'Core' }}</span>
            <span class="cpu-mhz">{{
              cpu_info[0]?.mhz > 0.01 ? `${(cpu_info[0]?.mhz / 1000).toFixed(2)}GHz` : 'Core'
            }}</span>
            * {{ cpu_info.length }}
          </p>
        </ACard>
      </ACol>
      <ACol
        :xl="10"
        :lg="16"
        :md="24"
        :xs="24"
        class="chart_dashboard"
      >
        <ACard
          :title="$gettext('Memory and Storage')"
          :bordered="false"
        >
          <ARow :gutter="[0, 16]">
            <ACol
              :xs="24"
              :sm="24"
              :md="8"
            >
              <RadialBarChart
                :name="$gettext('Memory')"
                :series="[memory.pressure]"
                :center-text="memory.used"
                :bottom-text="memory.total"
                colors="#36a3eb"
              />
            </ACol>
            <ACol
              :xs="24"
              :sm="12"
              :md="8"
            >
              <RadialBarChart
                :name="$gettext('Swap')"
                :series="[memory.swap_percent]"
                :center-text="memory.swap_used"
                :bottom-text="memory.swap_total"
                colors="#ff6385"
              />
            </ACol>
            <ACol
              :xs="24"
              :sm="12"
              :md="8"
            >
              <RadialBarChart
                :name="$gettext('Storage')"
                :series="[disk.percentage]"
                :center-text="disk.used"
                :bottom-text="disk.total"
                colors="#87d068"
              />
            </ACol>
          </ARow>
        </ACard>
      </ACol>
      <ACol
        :xl="7"
        :lg="8"
        :sm="24"
        :xs="24"
        class="chart_dashboard network-total"
      >
        <ACard
          :title="$gettext('Network Statistics')"
          :bordered="false"
        >
          <ARow :gutter="16">
            <ACol :span="12">
              <AStatistic
                :value="bytesToSize(net.last_recv)"
                :title="$gettext('Network Total Receive')"
              />
            </ACol>
            <ACol :span="12">
              <AStatistic
                :value="bytesToSize(net.last_sent)"
                :title="$gettext('Network Total Send')"
              />
            </ACol>
          </ARow>
        </ACard>
      </ACol>
    </ARow>
    <ARow
      :gutter="[{ xs: 0, sm: 16 }, 16]"
      class="row-two"
    >
      <ACol
        :xl="8"
        :lg="24"
        :md="24"
        :sm="24"
        :xs="24"
      >
        <ACard
          :title="$gettext('CPU Status')"
          :bordered="false"
        >
          <AStatistic
            :value="cpu"
            title="CPU"
          >
            <template #suffix>
              <span>%</span>
            </template>
          </AStatistic>
          <AreaChart
            :series="cpu_analytic_series"
            :y-formatter="cpu_formatter"
            :max="100"
          />
        </ACard>
      </ACol>
      <ACol
        :xl="8"
        :lg="12"
        :md="24"
        :sm="24"
        :xs="24"
      >
        <ACard
          :title="$gettext('Network')"
          :bordered="false"
        >
          <ARow :gutter="16">
            <ACol :span="12">
              <AStatistic
                :value="bytesToSize(net.recv)"
                :title="$gettext('Receive')"
              >
                <template #suffix>
                  <span>/s</span>
                </template>
              </AStatistic>
            </ACol>
            <ACol :span="12">
              <AStatistic
                :value="bytesToSize(net.sent)"
                :title="$gettext('Send')"
              >
                <template #suffix>
                  <span>/s</span>
                </template>
              </AStatistic>
            </ACol>
          </ARow>
          <AreaChart
            :series="net_analytic"
            :y-formatter="net_formatter"
          />
        </ACard>
      </ACol>
      <ACol
        :xl="8"
        :lg="12"
        :md="24"
        :sm="24"
        :xs="24"
      >
        <ACard
          :title="$gettext('Disk IO')"
          :bordered="false"
        >
          <ARow :gutter="16">
            <ACol :span="12">
              <AStatistic
                :value="disk_io.writes"
                :title="$gettext('Writes')"
              >
                <template #suffix>
                  <span>/s</span>
                </template>
              </AStatistic>
            </ACol>
            <ACol :span="12">
              <AStatistic
                :value="disk_io.reads"
                :title="$gettext('Reads')"
              >
                <template #suffix>
                  <span>/s</span>
                </template>
              </AStatistic>
            </ACol>
          </ARow>
          <AreaChart :series="disk_io_analytic" />
        </ACard>
      </ACol>
    </ARow>
  </div>
</template>

<style lang="less" scoped>
.first-row {
  .ant-card {
    min-height: 227px;

    p {
      margin-bottom: 8px;
    }
  }

  margin-bottom: 20px;
}

.ant-card {
  .ant-statistic {
    margin: 0 0 10px 10px
  }

  .chart {
    max-width: 800px;
    max-height: 350px;
  }

  .chart_dashboard {
    padding: 60px;

    .description {
      width: 120px;
      text-align: center
    }
  }

  @media (max-width: 512px) {
    margin: 10px 0;
    .chart_dashboard {
      padding: 20px;
    }
  }
}

.load-avg-describe {
  @media (max-width: 1600px) and (min-width: 1200px) {
    display: none;
  }
}

.os-info {
  @media (max-width: 1600px) and (min-width: 1200px) {
    display: none;
  }
}

.cpu-model {
  @media (max-width: 1790px) and (min-width: 1200px) {
    display: none;
  }
}

.cpu-mhz {
  @media (min-width: 1790px) or (max-width: 1200px) {
    display: none;
  }
}
</style>
