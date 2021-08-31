<template>
    <div class="std-selector" @click="visible=true">
        <a-input v-model="_key" disabled hidden/>
        <a-input
            v-model="M_value"
            disabled
        />
        <a-modal
            :mask="false"
            :visible="visible"
            cancel-text="取消"
            ok-text="选择"
            title="选择器"
            @cancel="visible=false"
            @ok="ok()"
            :width="600"
            destroyOnClose
        >
            <std-table
                :api="api"
                :columns="columns"
                :data_key="data_key"
                :disable_search="disable_search"
                :pithy="true"
                :get_params="get_params"
                :selectionType="selectionType"
                @selected="onSelect"
                @selectedRecord="r => {record = r}"
            />
        </a-modal>
    </div>
</template>

<script>

export default {
    name: 'StdSelector',
    components: {
        StdTable: () => import('@/components/StdDataDisplay/StdTable')
    },
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
        disable_search: {
            type: Boolean,
            default: false
        },
        get_params: {
            type: Object,
            default() {
                return {}
            }
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
        _key() {
            if (!this._key) {
                this.M_value = null
            }
        },
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
    .ant-input {
        margin: 0 10px 0 0;
        cursor: pointer;
    }
    .ant-input-disabled {
        background: unset;
        color: unset;
    }
}
</style>
