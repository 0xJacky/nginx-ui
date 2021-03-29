<template>
    <div>
        <a-row class="row-two">
            <a-col :lg="24" :sm="24">
                <a-card style="min-height: 400px" title="服务器状态">
                    <a-row>
                        <a-col :lg="12" :sm="24" class="chart">
                            <a-statistic :value="cpu" style="margin: 0 50px 10px 0" title="CPU">
                                <template v-slot:suffix>
                                    <span>%</span>
                                </template>
                            </a-statistic>
                            <p>运行时间 {{ uptime }}</p>
                            <p>系统负载 1min:{{ loadavg.Loadavg1 }}  5min:{{ loadavg.Loadavg5 }}
                                15min:{{ loadavg.Loadavg15 }}</p>
                            <line-chart :chart-data="cpu_analytic" :options="cpu_analytic.options" :height="150"/>
                        </a-col>
                        <a-col :lg="6" :sm="8" :xs="12" class="chart_dashboard">
                            <div>
                                <a-tooltip
                                    :title="'已使用: '+ memory_used + ' 缓存: ' + memory_cached + '  空闲:' + memory_free +
                                     '  物理内存: ' + memory_total">
                                    <a-progress :percent="memory_pressure" strokeColor="rgb(135, 208, 104)" type="dashboard" />
                                    <p class="description">实际内存占用</p>
                                </a-tooltip>
                            </div>
                        </a-col>
                        <a-col :lg="6" :sm="8" :xs="12" class="chart_dashboard">
                            <div>
                                <a-tooltip
                                    :title="'已使用: '+ disk_used + ' / 总共: ' + disk_total">
                                    <a-progress :percent="disk_percentage" type="dashboard" />
                                    <p class="description">存储空间</p>
                                </a-tooltip>
                            </div>
                        </a-col>
                    </a-row>
                </a-card>
            </a-col>
        </a-row>
    </div>
</template>

<script>
import LineChart from "@/components/Chart/LineChart"

export default {
    name: "DashBoard",
    components: {
        LineChart
    },
    data() {
        return {
            websocket: null,
            loading: true,
            stat: {},
            memory_pressure: 0,
            memory_used: "",
            memory_cached: "",
            memory_free: "",
            memory_total: "",
            cpu_analytic: {
                datasets: [{
                    label: 'cpu user',
                    borderColor: '#36a3eb',
                    backgroundColor: '#36a3eb',
                    pointRadius: 0,
                    data: [],
                }, {
                    label: 'cpu total',
                    borderColor: '#ff6385',
                    backgroundColor: '#ff6385',
                    pointRadius: 0,
                    data: [],
                }],
                options: {
                    responsive: true,
                    maintainAspectRatio:false,
                    responsiveAnimationDuration: 0, // 调整大小后的动画持续时间
                    elements: {
                        line: {
                            tension: 0 // 禁用贝塞尔曲线
                        }
                    },
                    scales: {
                        yAxes: [{
                            ticks: {
                                max: 100,
                                min: 0,
                                stepSize: 20,
                                display: true
                            }
                        }],
                        xAxes: [
                            {
                                type: "time",
                                time: {
                                    unit: 'minute',
                                }
                            }
                        ]
                    }
                },
            },
            cpu: 0,
            disk_percentage: 0,
            disk_total: "",
            disk_used: "",
            uptime: "",
            loadavg: {}
        }
    },
    created() {
        this.websocket = new WebSocket(this.getWebSocketRoot() + "/analytic?token="
            + btoa(this.$store.state.user.token))
        this.websocket.onmessage = this.wsOnMessage
        this.websocket.onopen = this.wsOpen
        this.websocket.onerror = this.wsOnError
    },
    destroyed() {
        window.clearInterval(window.InitSetInterval)
        this.websocket.close()
    },
    methods: {
        wsOpen() {
            window.InitSetInterval = setInterval(() => {
                this.websocket.send("ping")
            }, 1000)
        },
        wsOnError() {
            this.websocket = new WebSocket(this.getWebSocketRoot() + "/analytic?token="
                + btoa(this.$store.state.user.token))
        },
        wsOnMessage(m) {
            const r = JSON.parse(m.data)
            console.log(r)
            this.cpu = r.cpu_system + r.cpu_user
            const time = new Date()
            //this.cpu_analytic.labels.push(time)
            this.cpu_analytic.datasets[0].data
                .push({x: time, y: r.cpu_user})
            this.cpu_analytic.datasets[1].data
                .push({x: time, y: this.cpu})
            if (this.cpu_analytic.datasets[0].data.length > 30) {
                this.cpu_analytic.datasets[0].data.shift()
                this.cpu_analytic.datasets[1].data.shift()
            }
            this.cpu = this.cpu.toFixed(2)
            this.memory_pressure = r.memory_pressure
            this.memory_used = r.memory_used
            this.memory_cached = r.memory_cached
            this.memory_free = r.memory_free
            this.memory_total = r.memory_total
            this.disk_percentage = r.disk_percentage
            this.disk_used = r.disk_used
            this.disk_total = r.disk_total
            let uptime = Math.floor(r.uptime)
            let uptime_days = Math.floor(uptime / 86400)
            uptime -= uptime_days * 86400
            let uptime_hours = Math.floor(uptime / 3600)
            uptime -= uptime_hours * 3600
            this.uptime = uptime_days + 'd ' + uptime_hours + 'h ' +  Math.floor(uptime/60) + 'm'
            this.loadavg = r.loadavg
        }
    }
}
</script>

<style lang="less" scoped>
.ant-card {
    margin: 10px;

    .chart {
        max-height: 300px;
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

