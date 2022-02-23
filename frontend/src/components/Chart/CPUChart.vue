<template>
    <apexchart type="area" height="200" :options="chartOptions" :series="series" ref="chart"/>
</template>

<script>
import VueApexCharts from 'vue-apexcharts'
import Vue from 'vue'

Vue.use(VueApexCharts)
Vue.component('apexchart', VueApexCharts)
export default {
    name: 'CPUChart',
    props: {
        series: Array
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
                    type: 'area',
                    zoom: {
                        enabled: false
                    },
                    animations: {
                        enabled: false,
                    },
                    toolbar: {
                        show: false
                    },
                },
                colors: ['#ff6385', '#36a3eb'],
                fill: {
                    // type: ['solid', 'gradient'],
                    gradient: {
                        shade: 'light'
                    }
                    //colors:  ['#ff6385', '#36a3eb'],
                },
                dataLabels: {
                    enabled: false
                },
                stroke: {
                    curve: 'smooth',
                    width: 0,
                },
                xaxis: {
                    type: 'datetime',
                    labels: {datetimeUTC: false},
                },
                tooltip: {
                    enabled: false
                },
                yaxis: {
                    max: 100,
                    tickAmount: 4,
                    min: 0,
                },
                legend: {
                    onItemClick: {
                        toggleDataSeries: false
                    },
                    onItemHover: {
                        highlightDataSeries: false
                    },
                }
            },
        }
    },
}
</script>

<style scoped>

</style>
