<script setup lang="ts">
import gettext from '@/gettext'

const {$gettext, interpolate} = gettext

import StdDataEntry from '@/components/StdDataEntry'
import StdPagination from './StdPagination.vue'
import {nextTick, reactive, ref, watch} from 'vue'
import {useRoute, useRouter} from 'vue-router'
import {message} from 'ant-design-vue'

const props = defineProps({
    api: Object,
    columns: Array,
    data_key: {
        type: String,
        default: 'data'
    },
    disable_search: {
        type: Boolean,
        default: false
    },
    disable_add: {
        type: Boolean,
        default: false
    },
    edit_text: String,
    deletable: {
        type: Boolean,
        default: true
    },
    get_params: {
        type: Object,
        default() {
            return {}
        }
    },
    editable: {
        type: Boolean,
        default: true
    },
    selectionType: {
        type: String,
        default: 'checkbox',
        validator: function (value: string) {
            return ['checkbox', 'radio'].indexOf(value) !== -1
        }
    },
    pithy: {
        type: Boolean,
        default: false
    },
    scrollX: {
        type: [Number, Boolean],
        default: true
    },
    rowKey: {
        type: String,
        default: 'id'
    }
})


const data_source = ref([])
const loading = ref(true)
const pagination = reactive({
    total: 1,
    per_page: 10,
    current_page: 1,
    total_pages: 1
})
const route = useRoute()
let params = reactive({
    ...route.query,
    ...props.get_params
})
const selectedRowKeys = ref([])
const rowSelection = reactive({})

const searchColumns = getSearchColumns()
const pithyColumns = getPithyColumns()

get_list()

defineExpose({
    get_list
})

function destroy(id: any) {
    props.api!.destroy(id).then(() => {
        get_list()
        message.success(interpolate($gettext('Delete ID: %{id}'), {id: id}))
    }).catch((e: any) => {
        message.error($gettext(e?.message ?? 'Server error'))
    })
}

function get_list(page_num = null) {
    loading.value = true
    if (page_num) {
        params['page'] = page_num
    }
    props.api!.get_list(params).then((r: any) => {
        data_source.value = r.data

        if (r.pagination !== undefined) {
            Object.assign(pagination, r.pagination)
        }

        loading.value = false
    }).catch((e: any) => {
        message.error(e?.message ?? $gettext('Server error'))
    })
}

function stdChange(pagination: any, filters: any, sorter: any) {
    if (sorter) {
        params['order_by'] = sorter.field
        params['sort'] = sorter.order === 'ascend' ? 'asc' : 'desc'
        nextTick(() => {
            get_list()
        })
    }
}

function getSearchColumns() {
    let searchColumns: any = []
    props.columns!.forEach((column: any) => {
        if (column.search) {
            searchColumns.push(column)
        }
    })
    return searchColumns
}

function getPithyColumns() {
    if (props.pithy) {
        return props.columns!.filter((c: any, index: any, columns: any) => {
            return c.pithy === true && c.display !== false
        })
    }
    return props.columns!.filter((c: any, index: any, columns: any) => {
        return c.display !== false
    })
}

function checked(c: any) {
    params[c.target.value] = c.target.checked
}

function onSelectChange(_selectedRowKeys: any) {
    selectedRowKeys.value = _selectedRowKeys
    // this.$emit('selected', selectedRowKeys)
}

function onSelect(record: any) {
    // this.$emit('selectedRecord', record)
}

const router = useRouter()

const reset_search = async () => {
    Object.keys(params).forEach(v => {
        delete params[v]
    })
    router.push({query: {}}).catch(() => {
    })
}

watch(params, () => {
    router.push({query: params})
    get_list()
})
</script>

<template>
    <div class="std-table">
        <std-data-entry
            v-if="!disable_search"
            :data-list="searchColumns"
            v-model:data-source="params"
            layout="inline"
        >
            <template #action>
                <div class="reset-btn">
                    <a-button @click="reset_search">
                        <translate>Reset</translate>
                    </a-button>
                </div>
            </template>
        </std-data-entry>
        <a-table
            :columns="pithyColumns"
            :data-source="data_source"
            :loading="loading"
            :pagination="false"
            :row-key="rowKey"
            :rowSelection="{selectedRowKeys: selectedRowKeys, onChange: onSelectChange,
            onSelect: onSelect, type: selectionType}"
            @change="stdChange"
            :scroll="{ x: scrollX }"
        >
            <template
                v-slot:bodyCell="{text, record, index, column}"
            >
                <template v-if="column.dataIndex === 'action'">
                    <a v-if="props.editable" @click="$emit('clickEdit', record[props.rowKey], record)">
                        {{ props.edit_text || $gettext('Modify') }}
                    </a>
                    <slot name="actions" :record="record"/>
                    <template v-if="props.deletable">
                        <a-divider type="vertical"/>
                        <a-popconfirm
                            :cancelText="$gettext('No')"
                            :okText="$gettext('OK')"
                            :title="$gettext('Are you sure you want to delete ?')"
                            @confirm="destroy(record[rowKey])">
                            <a v-translate>Delete</a>
                        </a-popconfirm>
                    </template>
                </template>
            </template>
        </a-table>
        <std-pagination :pagination="pagination" @changePage="get_list"/>
    </div>
</template>

<style lang="less">
.ant-table-scroll {
    .ant-table-body {
        overflow-x: auto !important;
    }
}
</style>

<style lang="less" scoped>
.ant-form {
    margin: 10px 0 20px 0;
}

.ant-slider {
    min-width: 90px;
}

.std-table {
    .ant-table-wrapper {
        // overflow-x: scroll;
    }
}

.reset-btn {
    // min-height: 50px;
    height: 100%;
    display: flex;
    align-items: flex-end;
}
</style>
