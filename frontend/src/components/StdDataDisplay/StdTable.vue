<script setup lang="ts">
import gettext from '@/gettext'
import StdDataEntry from '@/components/StdDataEntry'
import StdPagination from './StdPagination.vue'
import {computed, onMounted, reactive, ref, watch} from 'vue'
import {useRoute, useRouter} from 'vue-router'
import {message} from 'ant-design-vue'
import {downloadCsv} from '@/lib/helper'
import dayjs from 'dayjs'
import Sortable from 'sortablejs'
import {HolderOutlined} from '@ant-design/icons-vue'
import {toRaw} from '@vue/reactivity'

const {$gettext, interpolate} = gettext

const emit = defineEmits(['onSelected', 'onSelectedRecord', 'clickEdit', 'update:selectedRowKeys', 'clickBatchModify'])

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
    disable_query_params: {
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
    },
    exportCsv: {
        type: Boolean,
        default: false
    },
    size: String,
    selectedRowKeys: {
        type: Array
    },
    useSortable: Boolean
})

const data_source: any = ref([])
const expand_keys_list: any = ref([])
const rows_key_index_map: any = ref({})

const loading = ref(true)
const pagination = reactive({
    total: 1,
    per_page: 10,
    current_page: 1,
    total_pages: 1
})

const route = useRoute()
const params = reactive({
    ...props.get_params
})

const selectedKeysLocalBuffer: any = ref([])

const selectedRowKeysBuffer = computed({
    get() {
        return props.selectedRowKeys || selectedKeysLocalBuffer.value
    },
    set(v) {
        selectedKeysLocalBuffer.value = v
        emit('update:selectedRowKeys', v)
    }
})

const searchColumns = getSearchColumns()
const pithyColumns = getPithyColumns()
const batchColumns = getBatchEditColumns()

onMounted(() => {
    if (!props.disable_query_params) {
        Object.assign(params, route.query)
    }
    get_list()

    if (props.useSortable) {
        initSortable()
    }
})

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

function get_list(page_num = null, page_size = 20) {
    loading.value = true
    if (page_num) {
        params['page'] = page_num
        params['page_size'] = page_size
    }
    props.api!.get_list(params).then(async (r: any) => {
        data_source.value = r.data
        rows_key_index_map.value = {}
        if (props.useSortable) {
            function buildIndexMap(data: any, level: number = 0, index: number = 0, total: number[] = []) {
                if (data && data.length > 0) {
                    data.forEach((v: any) => {
                        v.level = level
                        let current_index = [...total, index++]
                        rows_key_index_map.value[v.id] = current_index
                        if (v.children) buildIndexMap(v.children, level + 1, 0, current_index)
                    })
                }
            }

            buildIndexMap(r.data)
        }

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
        selectedRowKeysBuffer.value = []
        params['order_by'] = sorter.field
        params['sort'] = sorter.order === 'ascend' ? 'asc' : 'desc'
        switch (sorter.order) {
            case 'ascend':
                params['sort'] = 'asc'
                break
            case 'descend':
                params['sort'] = 'desc'
                break
            default:
                params['sort'] = null
                break
        }
    }
    if (pagination) {
        selectedRowKeysBuffer.value = []
    }
}

function expandedTable(keys: any) {
    expand_keys_list.value = keys
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

function getBatchEditColumns() {
    let batch: any = []
    props.columns!.forEach((column: any) => {
        if (column.batch) {
            batch.push(column)
        }
    })
    return batch
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

const crossPageSelect: any = {}

async function onSelectChange(_selectedRowKeys: any) {
    const page = params.page || 1

    crossPageSelect[page] = await _selectedRowKeys

    let t: any = []
    Object.keys(crossPageSelect).forEach(v => {
        t.push(...crossPageSelect[v])
    })
    const n: any = [..._selectedRowKeys]
    t = await t.concat(n)
    // console.log(crossPageSelect)
    const set = new Set(t)
    selectedRowKeysBuffer.value = Array.from(set)
    emit('onSelected', selectedRowKeysBuffer.value)
}

function onSelect(record: any) {
    emit('onSelectedRecord', record)
}

const router = useRouter()

const reset_search = async () => {
    Object.keys(params).forEach(v => {
        delete params[v]
    })

    Object.assign(params, {
        ...props.get_params
    })

    router.push({query: {}}).catch(() => {
    })
}

watch(params, () => {
    if (!props.disable_query_params) {
        router.push({query: params})
    }
    get_list()
})

const rowSelection = computed(() => {
    if (batchColumns.length > 0 || props.selectionType) {
        return {
            selectedRowKeys: selectedRowKeysBuffer.value, onChange: onSelectChange,
            onSelect: onSelect, type: batchColumns.length > 0 ? 'checkbox' : props.selectionType
        }
    } else {
        return null
    }
})

function fn(obj: Object, desc: string) {
    const arr: string[] = desc.split('.')
    while (arr.length) {
        // @ts-ignore
        const top = obj[arr.shift()]
        if (top === undefined) {
            return null
        }
        obj = top
    }
    return obj
}

async function export_csv() {
    let header = []
    let headerKeys: any[] = []
    const showColumnsMap: any = {}
    // @ts-ignore
    for (let showColumnsKey in pithyColumns) {
        // @ts-ignore
        if (pithyColumns[showColumnsKey].dataIndex === 'action') continue
        // @ts-ignore
        let t = pithyColumns[showColumnsKey].title

        if (typeof t === 'function') {
            t = t()
        }
        header.push({
            title: t,
            // @ts-ignore
            key: pithyColumns[showColumnsKey].dataIndex
        })
        // @ts-ignore
        headerKeys.push(pithyColumns[showColumnsKey].dataIndex)
        // @ts-ignore
        showColumnsMap[pithyColumns[showColumnsKey].dataIndex] = pithyColumns[showColumnsKey]
    }

    let dataSource: any = []
    let hasMore = true
    let page = 1
    while (hasMore) {
        // 准备 DataSource
        await props.api!.get_list({page}).then((response: any) => {
            if (response.data.length === 0) {
                hasMore = false
                return
            }
            if (response[props.data_key] === undefined) {
                dataSource = dataSource.concat(...response.data)
            } else {
                dataSource = dataSource.concat(...response[props.data_key])
            }
        }).catch((e: any) => {
            message.error(e.message ?? $gettext('Server error'))
            hasMore = false
            return
        })
        page += 1
    }
    const data: any[] = []
    dataSource.forEach((row: Object) => {
        let obj: any = {}
        headerKeys.forEach(key => {
            let data = fn(row, key)
            const c = showColumnsMap[key]
            data = c?.customRender?.({text: data}) ?? data
            obj[c.dataIndex] = data
        })
        data.push(obj)
    })

    downloadCsv(header, data,
        `${$gettext('Export')}-${dayjs().format('YYYYMMDDHHmmss')}.csv`)
}

const hasSelectedRow = computed(() => {
    return batchColumns.length > 0 && selectedRowKeysBuffer.value.length > 0
})

function click_batch_edit() {
    emit('clickBatchModify', batchColumns, selectedRowKeysBuffer.value)
}

function getLeastIndex(index: number) {
    return index >= 1 ? index : 1
}

function getTargetData(data: any, indexList: number[]): any {
    let target: any = {children: data}
    indexList.forEach((index: number) => {
        target.children[index].parent = target
        target = target.children[index]
    })
    return target
}

function initSortable() {
    const table: any = document.querySelector('#std-table tbody')
    new Sortable(table, {
        handle: '.ant-table-drag-icon',
        animation: 150,
        sort: true,
        forceFallback: true,
        setData: function (dataTransfer) {
            dataTransfer.setData('Text', '')
        },
        onStart({item}) {
            let targetRowKey = Number(item.dataset.rowKey)
            if (targetRowKey) {
                expand_keys_list.value = expand_keys_list.value.filter((item: number) => item !== targetRowKey)
            }
        },
        onMove({dragged, related}) {
            const oldRow: number[] = rows_key_index_map.value?.[Number(dragged.dataset.rowKey)]
            const newRow: number[] = rows_key_index_map.value?.[Number(related.dataset.rowKey)]
            if (oldRow.length !== newRow.length || oldRow[oldRow.length - 2] != newRow[newRow.length - 2]) {
                return false
            }
        },
        async onEnd({item, newIndex, oldIndex}) {
            if (newIndex === oldIndex) return

            const indexDelta: number = Number(oldIndex) - Number(newIndex)
            const direction: number = indexDelta > 0 ? +1 : -1

            let rowIndex: number[] = rows_key_index_map.value?.[Number(item.dataset.rowKey)]
            const newRow = getTargetData(data_source.value, rowIndex)
            const newRowParent = newRow.parent
            const level: number = newRow.level

            let currentRowIndex: number[] = [...rows_key_index_map.value?.
                [Number(table.children[Number(newIndex) + direction].dataset.rowKey)]]
            let currentRow: any = getTargetData(data_source.value, currentRowIndex)
            // Reset parent
            currentRow.parent = newRow.parent = null
            newRowParent.children.splice(rowIndex[level], 1)
            newRowParent.children.splice(currentRowIndex[level], 0, toRaw(newRow))

            let changeIds: number[] = []

            function processChanges(row: any, children: boolean = false, newIndex: number | undefined = undefined) {
                // Build changes ID list expect new row
                if (children || newIndex === undefined) changeIds.push(row.id)

                if (newIndex !== undefined)
                    rows_key_index_map.value[row.id][level] = newIndex
                else if (children)
                    rows_key_index_map.value[row.id][level] += direction

                row.parent = null
                if (row.children) {
                    row.children.forEach((v: any) => processChanges(v, true, newIndex))
                }
            }

            // Replace row index for new row
            processChanges(newRow, false, currentRowIndex[level])
            // Rebuild row index maps for changes row
            for (let i = Number(oldIndex); i != newIndex; i -= direction) {
                let rowIndex: number[] = rows_key_index_map.value?.[table.children[i].dataset.rowKey]
                rowIndex[level] += direction
                processChanges(getTargetData(data_source.value, rowIndex))
            }
            console.log('Change row id', newRow.id, 'order', newRow.id, '=>', currentRow.id, ', direction: ', direction,
                ', changes IDs:', changeIds)

            props.api!.update_order({
                target_id: newRow.id,
                direction: direction,
                affected_ids: changeIds
            }).then(() => {
                message.success($gettext('Updated successfully'))
            }).catch((e: any) => {
                message.error(e?.message ?? $gettext('Server error'))
            })
        }
    })
}


</script>

<template>
    <div class="std-table">
        <std-data-entry
            v-if="!disable_search && searchColumns.length"
            :data-list="searchColumns"
            v-model:data-source="params"
            layout="inline"
        >
            <template #action>
                <a-space class="action-btn">
                    <a-button v-if="exportCsv" @click="export_csv" type="primary" ghost>
                        {{ $gettext('Export') }}
                    </a-button>
                    <a-button @click="reset_search">
                        {{ $gettext('Reset') }}
                    </a-button>
                    <a-button v-if="hasSelectedRow" @click="click_batch_edit">
                        {{ $gettext('Batch Modify') }}
                    </a-button>
                </a-space>
            </template>
        </std-data-entry>
        <a-table
            :columns="pithyColumns"
            :data-source="data_source"
            :loading="loading"
            :pagination="false"
            :row-key="rowKey"
            :rowSelection="rowSelection"
            @change="stdChange"
            :scroll="{ x: scrollX }"
            :size="size"
            id="std-table"
            @expandedRowsChange="expandedTable"
            :expandedRowKeys="expand_keys_list"
        >
            <template
                v-slot:bodyCell="{text, record, index, column}"
            >
                <template v-if="column.handle === true">
                    <span class="ant-table-drag-icon"><HolderOutlined/></span>
                    {{ text }}
                </template>
                <template v-if="column.dataIndex === 'action'">
                    <a-button type="link" size="small" v-if="props.editable"
                              @click="$emit('clickEdit', record[props.rowKey], record)">
                        {{ props.edit_text || $gettext('Modify') }}
                    </a-button>
                    <slot name="actions" :record="record"/>
                    <template v-if="props.deletable">
                        <a-divider type="vertical"/>
                        <a-popconfirm
                            :cancelText="$gettext('No')"
                            :okText="$gettext('OK')"
                            :title="$gettext('Are you sure you want to delete?')"
                            @confirm="destroy(record[rowKey])">
                            <a-button type="link" size="small">{{ $gettext('Delete') }}</a-button>
                        </a-popconfirm>
                    </template>
                </template>
            </template>
        </a-table>
        <std-pagination :size="size" :pagination="pagination" @change="get_list" @changePageSize="stdChange"/>
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

.action-btn {
    // min-height: 50px;
    height: 100%;
    display: flex;
    align-items: flex-start;
}

:deep(.ant-form-inline .ant-form-item) {
    margin-bottom: 10px;
}
</style>

<style lang="less">
.ant-table-drag-icon {
    float: left;
    margin-right: 16px;
    cursor: grab;
}

.sortable-ghost *, .sortable-chosen * {
    cursor: grabbing !important;
}
</style>
