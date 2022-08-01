<script setup lang="ts">
import VueApexCharts from 'vue3-apexcharts'
import {ref, watch} from 'vue'
import {useSettingsStore} from '@/pinia'
import {storeToRefs} from 'pinia'

const {series, max, y_formatter} = defineProps(['series', 'max', 'y_formatter'])

const settings = useSettingsStore()
const {theme} = storeToRefs(settings)

const fontColor = () => {
    return theme.value === 'dark' ? '#b4b4b4' : undefined
}

const chart = ref(null)

let chartOptions = {
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
        max: max,
        tickAmount: 4,
        min: 0,
        labels: {
            style: {
                colors: fontColor()
            },
            formatter: y_formatter
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
}

let instance: ApexCharts | null = chart.value

const callback = () => {
    chartOptions = {
        ...chartOptions,
        ...{
            xaxis: {
                type: 'datetime',
                labels: {
                    datetimeUTC: false,
                    style: {
                        colors: fontColor()
                    }
                }
            },
            yaxis: {
                max: max,
                tickAmount: 4,
                min: 0,
                labels: {
                    style: {
                        colors: fontColor()
                    },
                    formatter: y_formatter
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
        }
    }
    instance!.updateOptions(chartOptions)
}


watch(theme, callback)
// watch(series, () => {
//     instance?.updateSeries(series)
// })
</script>

<template>
    <VueApexCharts type="area" height="200" :options="chartOptions" :series="series" ref="chart"/>
</template>


<style scoped>

</style>
