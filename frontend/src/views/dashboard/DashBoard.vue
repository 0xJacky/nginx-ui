<script setup lang="ts">
import AreaChart from '@/components/Chart/AreaChart.vue'

import RadialBarChart from '@/components/Chart/RadialBarChart.vue'
import {useGettext} from 'vue3-gettext'
import {onMounted, onUnmounted, reactive, ref} from 'vue'
import analytic from '@/api/analytic'
import ws from '@/lib/websocket'
import {bytesToSize} from '@/lib/helper'

const {$gettext} = useGettext()

const websocket = ws('/api/analytic')
websocket.onmessage = wsOnMessage

const host = reactive({})
const cpu = ref('0.0')
const cpu_info = reactive([])
const cpu_analytic_series = reactive([{name: 'User', data: <any>[]}, {name: 'Total', data: <any>[]}])
const net_analytic = reactive([{name: $gettext('Receive'), data: <any>[]},
    {name: $gettext('Send'), data: <any>[]}])
const disk_io_analytic = reactive([{name: $gettext('Writes'), data: <any>[]},
    {name: $gettext('Writes'), data: <any>[]}])
const memory = reactive({})
const disk = reactive({})
const disk_io = reactive({writes: 0, reads: 0})
const uptime = ref('')
const loadavg = reactive({})
const net = reactive({recv: 0, sent: 0, last_recv: 0, last_sent: 0})

const net_formatter = (bytes: number) => {
    return bytesToSize(bytes) + '/s'
}

interface Usage {
    x: number
    y: number
}

onMounted(() => {
    analytic.init().then(r => {
        Object.assign(host, r.host)
        Object.assign(cpu_info, r.cpu.info)
        Object.assign(memory, r.memory)
        Object.assign(disk, r.disk)

        net.last_recv = r.network.init.bytesRecv
        net.last_sent = r.network.init.bytesSent
        r.cpu.user.forEach((u: Usage) => {
            cpu_analytic_series[0].data.push([u.x, u.y.toFixed(2)])
        })
        r.cpu.total.forEach((u: Usage) => {
            cpu_analytic_series[1].data.push([u.x, u.y.toFixed(2)])
        })
        r.network.bytesRecv.forEach((u: Usage) => {
            net_analytic[0].data.push([u.x, u.y.toFixed(2)])
        })
        r.network.bytesSent.forEach((u: Usage) => {
            net_analytic[1].data.push([u.x, u.y.toFixed(2)])
        })
        disk_io_analytic[0].data = disk_io_analytic[0].data.concat(r.disk_io.writes)
        disk_io_analytic[1].data = disk_io_analytic[1].data.concat(r.disk_io.reads)

    })
})

onUnmounted(() => {
    websocket.close()
})

function wsOnMessage(m: { data: any }) {
    const r = JSON.parse(m.data)

    const cpu_usage = r.cpu.system + r.cpu.user
    cpu.value = cpu_usage.toFixed(2)

    const time = new Date().getTime()

    cpu_analytic_series[0].data.push([time, r.cpu.user.toFixed(2)])
    cpu_analytic_series[1].data.push([time, cpu.value])

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
    let _uptime = Math.floor(r.uptime)
    let uptime_days = Math.floor(_uptime / 86400)
    _uptime -= uptime_days * 86400
    let uptime_hours = Math.floor(_uptime / 3600)
    _uptime -= uptime_hours * 3600
    uptime.value = uptime_days + 'd ' + uptime_hours + 'h ' + Math.floor(_uptime / 60) + 'm'

    // loadavg
    Object.assign(loadavg, r.loadavg)

    // network
    Object.assign(net, r.network)
    net.recv = r.network.bytesRecv - net.last_recv
    net.sent = r.network.bytesSent - net.last_sent
    net.last_recv = r.network.bytesRecv
    net.last_sent = r.network.bytesSent

    net_analytic[0].data.push([time, net.recv])
    net_analytic[1].data.push([time, net.sent])

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
        <a-row :gutter="[16,16]" class="first-row">
            <a-col :xl="7" :lg="24" :md="24">
                <a-card :title="$gettext('Server Info')" :bordered="false">
                    <p>
                        <translate>Uptime:</translate>
                        {{ uptime }}
                    </p>
                    <p>
                        <translate>Load Averages:</translate>
                        1min:{{ loadavg?.load1?.toFixed(2) }} |
                        5min:{{ loadavg?.load5?.toFixed(2) }} |
                        15min:{{ loadavg?.load15?.toFixed(2) }}
                    </p>
                    <p>
                        <translate>OS:</translate>
                        {{ host.platform }} ({{ host.platformVersion }}
                        {{ host.os }} {{ host.kernelVersion }}
                        {{ host.kernelArch }})
                    </p>
                    <p v-if="cpu_info">
                        <translate>CPU:</translate>
                        {{ cpu_info[0]?.modelName }} * {{ cpu_info.length }}
                    </p>
                </a-card>
            </a-col>
            <a-col :xl="10" :lg="16" :md="24" class="chart_dashboard">
                <a-card :title="$gettext('Memory and Storage')" :bordered="false">
                    <a-row :gutter="[0,16]">
                        <a-col :xs="24" :sm="24" :md="8">
                            <radial-bar-chart :name="$gettext('Memory')" :series="[memory.pressure]"
                                              :centerText="memory.used" :bottom-text="memory.total" colors="#36a3eb"/>
                        </a-col>
                        <a-col :xs="24" :sm="12" :md="8">
                            <radial-bar-chart :name="$gettext('Swap')" :series="[memory.swap_percent]"
                                              :centerText="memory.swap_used"
                                              :bottom-text="memory.swap_total" colors="#ff6385"/>
                        </a-col>
                        <a-col :xs="24" :sm="12" :md="8">
                            <radial-bar-chart :name="$gettext('Storage')" :series="[disk.percentage]"
                                              :centerText="disk.used" :bottom-text="disk.total" colors="#87d068"/>
                        </a-col>
                    </a-row>
                </a-card>
            </a-col>
            <a-col :xl="7" :lg="8" :sm="24" class="chart_dashboard">
                <a-card :title="$gettext('Network Statistics')" :bordered="false">
                    <a-row :gutter="16">
                        <a-col :span="12">
                            <a-statistic :value="bytesToSize(net.last_recv)"
                                         :title="$gettext('Network Total Receive')"/>
                        </a-col>
                        <a-col :span="12">
                            <a-statistic :value="bytesToSize(net.last_sent)"
                                         :title="$gettext('Network Total Send')"/>
                        </a-col>
                    </a-row>
                </a-card>
            </a-col>
        </a-row>
        <a-row class="row-two" :gutter="[16,32]">
            <a-col :xl="8" :lg="24" :md="24" :sm="24">
                <a-card :title="$gettext('CPU Status')" :bordered="false">
                    <a-statistic :value="cpu" title="CPU">
                        <template v-slot:suffix>
                            <span>%</span>
                        </template>
                    </a-statistic>
                    <area-chart :series="cpu_analytic_series" :max="100"/>
                </a-card>
            </a-col>
            <a-col :xl="8" :lg="12" :md="24" :sm="24">
                <a-card :title="$gettext('Network')">
                    <a-row :gutter="16">
                        <a-col :span="12">
                            <a-statistic :value="bytesToSize(net.recv)"
                                         :title="$gettext('Receive')">
                                <template v-slot:suffix>
                                    <span>/s</span>
                                </template>
                            </a-statistic>
                        </a-col>
                        <a-col :span="12">
                            <a-statistic :value="bytesToSize(net.sent)" :title="$gettext('Send')">
                                <template v-slot:suffix>
                                    <span>/s</span>
                                </template>
                            </a-statistic>
                        </a-col>
                    </a-row>
                    <area-chart :series="net_analytic" :y_formatter="net_formatter"/>
                </a-card>
            </a-col>
            <a-col :xl="8" :lg="12" :md="24" :sm="24">
                <a-card :title="$gettext('Disk IO')">
                    <a-row :gutter="16">
                        <a-col :span="12">
                            <a-statistic :value="disk_io.writes"
                                         :title="$gettext('Writes')">
                                <template v-slot:suffix>
                                    <span>/s</span>
                                </template>
                            </a-statistic>
                        </a-col>
                        <a-col :span="12">
                            <a-statistic :value="disk_io.reads" :title="$gettext('Reads')">
                                <template v-slot:suffix>
                                    <span>/s</span>
                                </template>
                            </a-statistic>
                        </a-col>
                    </a-row>
                    <area-chart :series="disk_io_analytic"/>
                </a-card>
            </a-col>
        </a-row>
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
        margin: 0 50px 10px 10px
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
</style>

