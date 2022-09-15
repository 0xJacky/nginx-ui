<script setup lang="ts">
import {onMounted, reactive, ref, watch} from 'vue'
import StdTable from '@/components/StdDataDisplay/StdTable.vue'

const props = defineProps(['selectedKey', 'value', 'recordValueIndex',
    'selectionType', 'api', 'columns', 'data_key',
    'disable_search', 'get_params', 'description'])
const emit = defineEmits(['update:selectedKey', 'changeSelect'])
const visible = ref(false)
const M_value = ref('')

onMounted(() => {
    init()
})

const selected = ref([])

const record: any = reactive({})

function init() {
    if (props.selectedKey && !props.value && props.selectionType === 'radio') {
        props.api.get(props.selectedKey).then((r: any) => {
            Object.assign(record, r)
            M_value.value = r[props.recordValueIndex]
        })
    }
}

function show() {
    visible.value = true
}

function onSelect(_selected: any) {
    selected.value = _selected
}

function onSelectedRecord(r: any) {
    Object.assign(record, r)
}

function ok() {
    visible.value = false
    if (props.selectionType == 'radio') {
        emit('update:selectedKey', selected.value[0])
    } else {
        emit('update:selectedKey', selected.value)
    }
    M_value.value = record[props.recordValueIndex]
    emit('changeSelect', record)
}

watch(props, () => {
    if (!props?.selectedKey) {
        M_value.value = ''
    } else if (props.value) {
        M_value.value = props.value
    } else {
        init()
    }
})
</script>

<template>
    <div class="std-selector-container">
        <div class="std-selector" @click="show()">
            <a-input v-model="selectedKey" disabled hidden/>
            <div class="value">
                {{ M_value }}
            </div>
            <a-modal
                    :mask="false"
                    :visible="visible"
                    cancel-text="取消"
                    ok-text="选择"
                    title="选择器"
                    @cancel="visible=false"
                    @ok="ok()"
                    :width="800"
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
                        @onSelected="onSelect"
                        @onSelectedRecord="onSelectedRecord"
                />
            </a-modal>
        </div>
    </div>
</template>

<style lang="less" scoped>
.std-selector-container {
    height: 39.9px;
    display: flex;
    align-items: center;

    .std-selector {
        box-sizing: border-box;
        font-variant: tabular-nums;
        list-style: none;
        font-feature-settings: 'tnum';
        height: 32px;
        padding: 4px 11px;
        color: rgba(0, 0, 0, 0.85);
        font-size: 14px;
        line-height: 1.5;
        background-color: #fff;
        background-image: none;
        border: 1px solid #d9d9d9;
        border-radius: 4px;
        transition: all 0.3s;
        margin: 0 10px 0 0;
        cursor: pointer;
        min-width: 180px;

        @media (prefers-color-scheme: dark) {
            background-color: #1e1f20;
            border: 1px solid #666666;
            color: rgba(255, 255, 255, 0.99);
        }

        .value {

        }
    }
}
</style>