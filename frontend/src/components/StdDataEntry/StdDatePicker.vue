<template>
    <a-date-picker
        v-model="dateModel"
        :show-time="showTime"
        @change="r => {changeDate(r.format())}"
    />
</template>

<script>
import moment from 'moment'
import 'moment/locale/zh-cn'

moment.locale('zh-cn')

export default {
    name: 'StdDatePicker',
    props: {
        date: String,
        showTime: {
            type: [Object, Boolean],
            default() {
                return false
            }
        }

    },
    model: {
        prop: 'date',
        event: 'changeDate'
    },
    data() {
        return {
            moment,
            dateModel: this.date ? moment(this.date) : null
        }
    },
    watch: {
        date() {
            this.dateModel = this.date ? moment(this.date) : null
        }
    },
    methods: {
        changeDate(d) {
            this.$emit('changeDate', d)
        }
    }
}
</script>

<style scoped>

</style>
