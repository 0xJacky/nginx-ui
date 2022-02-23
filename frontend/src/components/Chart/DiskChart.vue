<template>
    <apexchart type="area" height="200" :options="chartOptions" :series="series" ref="chart"/>
</template>

<script>
import VueApexCharts from 'vue-apexcharts'
import Vue from 'vue'

Vue.use(VueApexCharts)
Vue.component('apexchart', VueApexCharts)

const fontColor = () => {
    return window.matchMedia('(prefers-color-scheme: dark)').matches ? '#b4b4b4' : null
}
export default {
    name: 'DiskChart',
    props: {
        series: Array
    },
    watch: {
        series: {
            deep: true,
            handler() {
                this.$refs.chart.updateSeries(this.series)
            }
        },
    },
    mounted() {
        let media = window.matchMedia('(prefers-color-scheme: dark)')
        let callback = () => {
            this.chartOptions.xaxis = {
                type: 'datetime',
                    labels: {
                    datetimeUTC: false,
                        style: {
                        colors: fontColor()
                    }
                }
            }
            this.chartOptions.yaxis = {
                tickAmount: 3,
                    min: 0,
                    labels: {
                    style: {
                        colors: fontColor()
                    }
                }
            }
            this.chartOptions.legend = {
                labels: {
                    colors: fontColor()
                },
                onItemClick: {
                    toggleDataSeries: false
                },
                onItemHover: {
                    highlightDataSeries: false
                },
            }
            this.$refs.chart.updateOptions(this.chartOptions)
        }
        if (typeof media.addEventListener === 'function') {
            media.addEventListener('change', callback)
        } else if (typeof media.addListener === 'function') {
            media.addListener(callback)
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
                    labels: {
                        datetimeUTC: false,
                        style: {
                            colors: fontColor()
                        }
                    }
                },
                tooltip: {
                    enabled: false
                },
                yaxis: {
                    tickAmount: 3,
                    min: 0,
                    labels: {
                        style: {
                            colors: fontColor()
                        }
                    }
                },
                legend: {
                    labels: {
                        colors: fontColor()
                    },
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
