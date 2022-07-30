<template>
    <div>
        <a-checkbox-group v-model="checkedList" :options="options" @change="onChange"/>
        <template v-if="allowOther&&checkedList.indexOf('其他')>0">
            <a-form-item label="其他">
                <a-input v-model="other" @change="onChangeOther"/>
            </a-form-item>
        </template>
    </div>
</template>
<script>
export default {
    name: 'StdCheckGroup',
    props: {
        options: Array,
        allowOther: Boolean,
        data: {
            type: Object,
            default() {
                return {
                    checkedList: [],
                    other: ''
                }
            }
        }
    },
    model: {
        prop: 'data',
        event: 'changeData'
    },
    watch: {
        data() {
            this.checkedList = this.data.checkedList
            this.other = this.data.other
        }
    },
    data() {
        return {
            checkedList: this.data.checkedList,
            other: this.data.other
        }
    },
    methods: {
        onChange(checkedList) {
            this.checkedList = checkedList
            this.$emit('changeData', this.$data)
        },
        onChangeOther() {
            this.$emit('changeData', this.$data)
        }
    },
}
</script>
