<script setup lang="ts">
import VueApexCharts from 'vue3-apexcharts'
import {reactive} from 'vue'

const {series, centerText, colors, name, bottomText}
    = defineProps(['series', 'centerText', 'colors', 'name', 'bottomText'])

const chartOptions = reactive({
    series: series,
    chart: {
        type: 'radialBar',
        offsetY: 0
    },
    plotOptions: {
        radialBar: {
            startAngle: -135,
            endAngle: 135,
            dataLabels: {
                name: {
                    fontSize: '14px',
                    color: colors,
                    offsetY: 36
                },
                value: {
                    offsetY: 50,
                    fontSize: '14px',
                    color: undefined,
                    formatter: () => {
                        return ''
                    }
                }
            }
        }
    },
    fill: {
        colors: colors
    },
    labels: [name],
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
})
</script>

<template>
    <div class="radial-bar-container">
        <p class="text">{{ centerText }}</p>
        <p class="bottom_text">{{ bottomText }}</p>
        <VueApexCharts v-if="centerText" class="radialBar" type="radialBar" height="205" :options="chartOptions"
                       :series="series"
                       ref="chart"/>
    </div>
</template>


<style lang="less" scoped>
.radial-bar-container {
    position: relative;
    margin: 0 auto;
    height: 112px !important;

    .radialBar {
        position: absolute;
        top: -30px;
        @media (max-width: 768px) and (min-width: 290px) {
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
