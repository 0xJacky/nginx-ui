<template>
    <div>
        <a-row class="row-two">
            <a-col :lg="24" :sm="24">
                <a-card style="min-height: 250px" title="后端服务器实时数据">
                    <a-row>
                        <a-col :lg="12" :sm="24" class="chart">
                            <a-statistic :value="cpu" style="margin: 0 50px 10px 0" title="CPU">
                                <template v-slot:suffix>
                                    <span>%</span>
                                </template>
                            </a-statistic>
                            <mini-smooth-area :data-source="cpu_analytic"/>
                        </a-col>
                        <a-col :lg="6" :sm="10" class="chart">
                            <span>实际内存占用</span>
                            <div>
                                <a-tooltip
                                    :title="'已使用: '+ memory_used + ' / 总共: ' + memory_total">
                                    <a-progress :percent="memory_pressure" strokeColor="rgb(135, 208, 104)" type="circle"/>
                                </a-tooltip>
                            </div>
                        </a-col>
                        <a-col :lg="6" :sm="10" class="chart">
                            <span>存储空间</span>
                            <div>
                                <a-tooltip
                                    :title="'已使用: '+ disk_used + ' / 总共: ' + disk_total">
                                    <a-progress :percent="disk_percentage" type="circle"/>
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
import MiniSmoothArea from '@/components/Charts/MiniSmoothArea'
import Vue from 'vue'
import Viser from 'viser-vue'

Vue.use(Viser)

export default {
    name: "DashBoard",
    components: {
        MiniSmoothArea
    },
    data() {
        return {
            websocket: null,
            loading: true,
            stat: {},
            memory_pressure: 0,
            memory_used: "",
            memory_total: "",
            cpu_analytic: [],
            cpu: 0,
            disk_percentage: 0,
            disk_total: "",
            disk_used: "",
        }
    },
    created() {
        this.websocket = new WebSocket(process.env["VUE_APP_API_WSS_ROOT"] + "/analytic")
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
            this.websocket = new WebSocket(process.env["VUE_APP_API_WSS_ROOT"] + "/analytic")
        },
        wsOnMessage(m) {
            const r = JSON.parse(m.data)
            console.log(r)
            this.cpu = r.cpu_system + r.cpu_user
            this.cpu_analytic.push({x: new Date(), y: this.cpu})
            if (this.cpu_analytic.length > 30) {
                this.cpu_analytic.shift()
            }
            this.cpu = this.cpu.toFixed(2)
            this.memory_pressure = r.memory_pressure
            this.memory_used = r.memory_used
            this.memory_total = r.memory_total
            this.disk_percentage = r.disk_percentage
            this.disk_used = r.disk_used
            this.disk_total = r.disk_total
        }
    }
}
</script>

<style lang="less" scoped>
.ant-card {
    margin: 10px;
    @media (max-width: 512px) {
        margin: 10px 0;
    }

    .chart-card-content, .chart-wrapper, .chart {
        overflow: hidden;
    }
}

.row-two {
    .ant-card-body {
        min-height: 255px;
    }
}

.row-three {
    .ant-card {
        min-height: 377px;
    }
}
</style>

