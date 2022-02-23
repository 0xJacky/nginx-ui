<template>
    <div>
        <a-row :gutter="[16,16]" class="first-row">
            <a-col :lg="7" :md="24">
                <a-card :title="$gettext('Server Info')">
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
                    <p>
                        <translate>CPU:</translate>
                        {{ cpu_info[0]?.modelName }} * {{ cpu_info.length }}
                    </p>
<!--                    <p><translate>Memory</translate>: {{-->
<!--                            $gettextInterpolate(-->
<!--                                $gettext('Used: %{u}, Cached: %{c}, Free: %{f}, Physical Memory: %{p}'),-->
<!--                                {u: memory_used, c: memory_cached, f: memory_free, p: memory_total})-->
<!--                        }}</p>-->
<!--                    <p><translate>Storage</translate>: {{-->
<!--                            $gettextInterpolate($gettext('Used: %{used} / Total: %{total}'),-->
<!--                                {used: disk_used, total: disk_total})-->
<!--                        }}-->
<!--                    </p>-->
                </a-card>
            </a-col>
            <a-col :lg="12" :md="24" class="chart_dashboard">
                <a-card>
                    <a-row>
                        <a-col :xs="24" :sm="24" :md="8">
                            <radial-bar-chart :name="$gettext('Memory')" :series="[memory_pressure]"
                                              :centerText="memory_used" colors="#36a3eb"/>
                        </a-col>
                        <a-col :xs="24" :sm="12" :md="8">
                            <radial-bar-chart :name="$gettext('Swap')" :series="[memory_swap_percent]"
                                              :centerText="memory_swap_used" colors="#ff6385"/>
                        </a-col>
                        <a-col :xs="24" :sm="12" :md="8">
                            <radial-bar-chart :name="$gettext('Storage')" :series="[disk_percentage]"
                                              :centerText="disk_used" colors="#87d068"/>
                        </a-col>
                    </a-row>
                </a-card>
            </a-col>
            <a-col :lg="5" :sm="24" class="chart_dashboard">
                <a-card>
                    <a-row :gutter="16">
                        <a-col :span="24">
                            <a-statistic :value="bytesToSize(net.last_recv)"
                                         :title="$gettext('Network Total Receive')"/>
                        </a-col>
                        <a-col :span="24">
                            <a-statistic :value="bytesToSize(net.last_sent)"
                                         :title="$gettext('Network Total Send')" />
                        </a-col>
                    </a-row>
                </a-card>
            </a-col>
        </a-row>
        <a-row class="row-two" :gutter="[16,32]">
            <a-col :lg="8" :md="24" :sm="24">
                <a-card :title="$gettext('CPU Status')">
                    <a-statistic :value="cpu" title="CPU">
                        <template v-slot:suffix>
                            <span>%</span>
                        </template>
                    </a-statistic>
                    <c-p-u-chart :series="cpu_analytic_series"/>
                </a-card>
            </a-col>
            <a-col :lg="8" :md="24" :sm="24">
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
                    <net-chart :series="net_analytic"/>
                </a-card>
            </a-col>
            <a-col :lg="8" :md="24" :sm="24">
                <a-card :title="$gettext('Disk IO')">
                    <a-row :gutter="16">
                        <a-col :span="12">
                            <a-statistic :value="diskIO.writes"
                                         :title="$gettext('Writes')">
                                <template v-slot:suffix>
                                    <span>/s</span>
                                </template>
                            </a-statistic>
                        </a-col>
                        <a-col :span="12">
                            <a-statistic :value="diskIO.reads" :title="$gettext('Reads')">
                                <template v-slot:suffix>
                                    <span>/s</span>
                                </template>
                            </a-statistic>
                        </a-col>
                    </a-row>
                    <disk-chart :series="diskIO_analytic"/>
                </a-card>
            </a-col>
        </a-row>

    </div>
</template>

<script>
import ReconnectingWebSocket from 'reconnecting-websocket'
import CPUChart from '@/components/Chart/CPUChart'
import NetChart from '@/components/Chart/NetChart'
import $gettext from '@/lib/translate/gettext'
import RadialBarChart from '@/components/Chart/RadialBarChart'
import DiskChart from '@/components/Chart/DiskChart'

export default {
    name: 'DashBoard',
    components: {
        DiskChart,
        RadialBarChart,
        NetChart,
        CPUChart,
    },
    data() {
        return {
            websocket: null,
            loading: true,
            stat: {},
            memory_pressure: 0,
            memory_used: '',
            memory_cached: '',
            memory_free: '',
            memory_total: '',
            cpu_analytic_series: [{
                name: 'CPU User',
                data: []
            }, {
                name: 'CPU Total',
                data: []
            }],
            cpu: 0,
            memory_swap_used: '',
            memory_swap_percent: 0,
            disk_percentage: 0,
            disk_total: '',
            disk_used: '',
            net: {
                recv: 0,
                sent: 0,
                last_recv: 0,
                last_sent: 0,
            },
            diskIO: {
                writes: 0,
                reads: 0,
            },
            net_analytic: [{
                name: $gettext('Receive'),
                data: []
            }, {
                name: $gettext('Send'),
                data: []
            }],
            diskIO_analytic: [{
                name: $gettext('Writes'),
                data: []
            }, {
                name: $gettext('Reads'),
                data: []
            }],
            uptime: '',
            loadavg: {},
            cpu_info: [],
            host: {}
        }
    },
    created() {
        this.websocket = new ReconnectingWebSocket(this.getWebSocketRoot() + '/analytic?token='
            + btoa(this.$store.state.user.token))
        this.websocket.onmessage = this.wsOnMessage
        this.websocket.onopen = this.wsOpen
        this.$api.analytic.init().then(r => {
            this.cpu_info = r.cpu.info
            this.net.last_recv = r.network.init.bytesRecv
            this.net.last_sent = r.network.init.bytesSent
            this.host = r.host
            r.cpu.user.forEach(u => {
                this.cpu_analytic_series[0].data.push([u.x, u.y.toFixed(2)])
            })
            r.cpu.total.forEach(u => {
                this.cpu_analytic_series[1].data.push([u.x, u.y.toFixed(2)])
            })
            r.network.bytesRecv.forEach(u => {
                this.net_analytic[0].data.push([u.x, u.y.toFixed(2)])
            })
            r.network.bytesSent.forEach(u => {
                this.net_analytic[1].data.push([u.x, u.y.toFixed(2)])
            })
            this.diskIO_analytic[0].data = this.diskIO_analytic[0].data.concat(r.diskIO.writes)
            this.diskIO_analytic[1].data = this.diskIO_analytic[1].data.concat(r.diskIO.reads)
        })
    },
    destroyed() {
        this.websocket.close()
    },
    methods: {
        wsOpen() {
            this.websocket.send('ping')
        },
        wsOnMessage(m) {
            const r = JSON.parse(m.data)
            // console.log(r)
            this.cpu = r.cpu_system + r.cpu_user
            this.cpu = this.cpu.toFixed(2)
            const time = new Date().getTime()

            this.cpu_analytic_series[0].data.push([time, r.cpu_user.toFixed(2)])
            this.cpu_analytic_series[1].data.push([time, this.cpu])

            if (this.cpu_analytic_series[0].data.length > 100) {
                this.cpu_analytic_series[0].data.shift()
                this.cpu_analytic_series[1].data.shift()
            }

            // mem
            this.memory_pressure = r.memory_pressure
            this.memory_used = r.memory_used
            this.memory_cached = r.memory_cached
            this.memory_free = r.memory_free
            this.memory_total = r.memory_total
            this.memory_swap_percent = r.memory_swap_percent
            this.memory_swap_used = r.memory_swap_used

            // disk
            this.disk_percentage = r.disk_percentage
            this.disk_used = r.disk_used
            this.disk_total = r.disk_total

            let uptime = Math.floor(r.uptime)
            let uptime_days = Math.floor(uptime / 86400)
            uptime -= uptime_days * 86400
            let uptime_hours = Math.floor(uptime / 3600)
            uptime -= uptime_hours * 3600
            this.uptime = uptime_days + 'd ' + uptime_hours + 'h ' + Math.floor(uptime / 60) + 'm'
            this.loadavg = r.loadavg

            // net
            this.net.recv = r.network.bytesRecv - this.net.last_recv
            this.net.sent = r.network.bytesSent - this.net.last_sent
            this.net.last_recv = r.network.bytesRecv
            this.net.last_sent = r.network.bytesSent

            this.net_analytic[0].data.push([time, this.net.recv])
            this.net_analytic[1].data.push([time, this.net.sent])

            if (this.net_analytic[0].data.length > 100) {
                this.net_analytic[1].data.shift()
                this.net_analytic[0].data.shift()
            }

            // diskIO
            this.diskIO.writes = r.diskIO.writes.y
            this.diskIO.reads = r.diskIO.reads.y

            this.diskIO_analytic[0].data.push(r.diskIO.writes)
            this.diskIO_analytic[1].data.push(r.diskIO.reads)

            if (this.diskIO_analytic[0].data.length > 100) {
                this.diskIO_analytic[0].data.shift()
                this.diskIO_analytic[1].data.shift()
            }
        }
    }
}
</script>

<style lang="less" scoped>
.first-row {
    .ant-card {
        min-height: 227px;
    }
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

