<script>
import {Line, mixins} from 'vue-chartjs'

const {reactiveProp} = mixins

export default {
    name: 'LineChart',
    extends: Line,
    mixins: [reactiveProp],
    props: ['options'],
    data() {
        return {
            updating: false
        }
    },
    mounted() {
        this.renderChart(this.chartData, this.options)
    },
    watch: {
        chartData: {
            deep: true,
            handler() {
                if (!this.updating && this.$data && this.$data._chart) {
                    // Update the chart
                    this.updating = true
                    this.$data._chart.update()
                    this.$nextTick(() => this.updating = false)
                }
            }
        }
    }
}
</script>

<style lang="less" scoped>

</style>
