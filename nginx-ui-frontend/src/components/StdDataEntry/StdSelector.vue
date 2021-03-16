<template>
    <div class="std-selector">
        <a-input v-model="_key" disabled hidden/>
        <a-input v-model="M_value" disabled/>
        <a-button @click="visible=true">更变</a-button>
        <a-modal
            :mask="false"
            :visible="visible"
            cancel-text="取消"
            ok-text="选择"
            title="选择器"
            @cancel="visible=false"
            @ok="ok()"
        >
            <std-table
                :api="api"
                :columns="columns"
                :data_key="data_key"
                :disable_search="disable_search"
                :pagination_method="pagination_method"
                :pithy="true"
                :selectionType="selectionType"
                @selected="onSelect"
                @selectedRecord="r => {record = r}"
            />
        </a-modal>
    </div>
</template>

<script>
import StdTable from '@/components/StdDataDisplay/StdTable'

export default {
    name: 'StdSelector',
    components: {StdTable},
    props: {
        _key: [Number, String],
        value: String,
        recordValueIndex: [Number, String],
        selectionType: {
            type: String,
            default: 'checkbox',
            validator: function (value) {
                return ['checkbox', 'radio'].indexOf(value) !== -1
            }
        },
        api: Object,
        columns: Array,
        data_key: String,
        pagination_method: {
            type: String,
            validator: function (value) {
                return ['a', 'b'].indexOf(value) !== -1
            }
        },
        disable_search: {
            type: Boolean,
            default: true
        }
    },
    model: {
        prop: '_key',
        event: 'changeSelect'
    },
    data() {
        return {
            visible: false,
            selected: [],
            record: {},
            M_value: this.value
        }
    },
    watch: {
        value() {
            this.M_value = this.value
        }
    },
    methods: {
        onSelect(selected) {
            this.selected = selected
        },
        ok() {
            this.visible = false
            let selected = this.selected
            if (this.selectionType === 'radio') {
                selected = this.selected[0]
            }
            this.M_value = this.record[this.recordValueIndex]
            this.$emit('changeSelect', selected)
        }
    }
}
</script>

<style lang="less" scoped>
.std-selector {
    min-width: 300px;

    .ant-input {
        width: auto;
        margin: 0 10px 0 0;
    }
}
</style>
