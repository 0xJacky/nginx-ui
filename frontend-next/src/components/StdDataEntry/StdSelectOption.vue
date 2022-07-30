<template>
    <a-select
        v-model="tempValue"
        :defaultValue="Object.keys(options)[0]"
        @change="$emit('change', isNaN(parseInt(tempValue)) || keyType === 'string' ? tempValue : parseInt(tempValue) )">
        <a-select-option v-for="(v,k) in options" :key="k">{{ v }}</a-select-option>
    </a-select>
</template>

<script>
export default {
    name: 'StdSelectOption',
    props: {
        value: [Number, String, Boolean],
        options: [Array, Object],
        keyType: {
            type: String,
            default() {
                return 'int'
            }
        }
    },
    model: {
        prop: 'value',
        event: 'change'
    },
    data() {
        return {
            tempValue: null
        }
    },
    watch: {
        value() {
            this.tempValue = this.value != null ? this.value.toString() : null
        }
    },
    created() {
        this.tempValue = this.value != null ? this.value.toString() : null
    }
}
</script>

<style lang="less" scoped>
.ant-select {
    min-width: 80px;
}
</style>
