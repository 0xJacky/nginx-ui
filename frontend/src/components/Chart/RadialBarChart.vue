<template>
    <div class="container">
        <p class="text">{{ centerText }}</p>
        <p class="bottom_text">{{ bottomText }}</p>
        <apexchart class="radialBar" type="radialBar" height="205" :options="chartOptions" :series="series" ref="chart"/>
    </div>
</template>

<script>
import VueApexCharts from 'vue-apexcharts'
import Vue from 'vue'

Vue.use(VueApexCharts)
Vue.component('apexchart', VueApexCharts)
export default {
    name: 'RadialBarChart',
    props: {
        series: Array,
        centerText: String,
        colors: String,
        name: String,
        bottomText: String,
    },
    watch: {
        series: {
            deep: true,
            handler() {
                this.$refs.chart.updateSeries(this.series)
            }
        }
    },
    data() {
        return {
            chartOptions: {
                series: this.series,
                chart: {
                    type: 'radialBar',
                    offsetY: -30
                },
                plotOptions: {
                    radialBar: {
                        startAngle: -135,
                        endAngle: 135,
                        dataLabels: {
                            name: {
                                fontSize: '14px',
                                color: this.colors,
                                offsetY: 36
                            },
                            value: {
                                offsetY: 50,
                                fontSize: '14px',
                                color: undefined,
                                formatter: () => {return ''}
                            }
                        }
                    }
                },
                fill: {
                    colors: this.colors
                },
                labels: [this.name],
                states: {
                    hover: {
                        filter: {
                            type: 'none'
                        }
                    },
                    active: {
                        filter: {
                            type: 'none'
                        }
                    }
                }
            }
        }
    }
}
</script>

<style lang="less" scoped>
.container {
    position: relative;
    margin: 0 auto;
    height: 112px!important;
    .radialBar {
        position: absolute;
        top: -30px;
        @media(max-width: 768px) and (min-width: 290px) {
            left: 50%;
            transform: translateX(-50%);
        }
    }
    .text {
        position: absolute;
        top: calc(50% - 5px);
        width: 100%;
        text-align: center;
    }
    .bottom_text {
        position: absolute;
        top: calc(106px);
        font-weight: 600;
        width: 100%;
        text-align: center;
    }
}
</style>
