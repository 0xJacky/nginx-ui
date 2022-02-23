<template>
    <div class="container">
        <p class="text">{{ centerText }}</p>
        <apexchart type="radialBar" height="205" :options="chartOptions" :series="series" ref="chart"/>
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
                    offsetY: -10
                },
                plotOptions: {
                    radialBar: {
                        startAngle: -135,
                        endAngle: 135,
                        dataLabels: {
                            name: {
                                fontSize: '15px',
                                color: this.colors,
                                offsetY: 56
                            },
                            value: {
                                offsetY: 60,
                                fontSize: '14px',
                                color: undefined,
                                formatter: function (val) {
                                    return val + "%";
                                }
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
    .text {
        position: absolute;
        top: calc(72px);
        width: 100%;
        text-align: center;

    }
}
</style>
