<template>
    <div class="std-selector" @click="show()">
        <a-input v-model="_key" disabled hidden/>
        <div class="value">
            <p>{{ M_value }}</p>
        </div>
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
            {{ description }}
            <std-table
                :api="api"
                :columns="columns"
                :data_key="data_key"
                :disable_search="disable_search"
                :pithy="true"
                :get_params="get_params"
                :selectionType="selectionType"
                :disable_query_params="true"
                @selected="onSelect"
                @selectedRecord="onSelectedRecord"
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
        },
        description: String
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
        show() {
            this.visible = true
        },
        onSelect(selected) {
            this.selected = selected
        },
        onSelectedRecord(r) {
            this.record = r
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

<style scoped>
.ant-form-inline .std-selector {
    height: 40px;
}
</style>

<style lang="less" scoped>
.std-selector {
    height: 38px;
    min-width: 180px;
    position: relative;

    .value {
        box-sizing: border-box;
        font-variant: tabular-nums;
        list-style: none;
        font-feature-settings: 'tnum';
        position: absolute;
        top: 50%;
        bottom: 50%;
        left: 50%;
        -webkit-transform: translateX(-50%) translateY(-50%);
        display: inline-block;
        width: 100%;
        height: 32px;
        padding: 4px 11px;
        color: rgba(0, 0, 0, 0.65);
        font-size: 14px;
        line-height: 1.5;
        background-color: #fff;
        background-image: none;
        border: 1px solid #d9d9d9;
        border-radius: 4px;
        transition: all 0.3s;
        margin: 0 10px 0 0;
        cursor: pointer;
        @media (prefers-color-scheme: dark) {
            background-color: #1e1f20;
            border: 1px solid #666666;
            color: rgba(255, 255, 255, 0.99);
        }
    }
}
</style>
