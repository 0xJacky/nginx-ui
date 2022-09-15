<script setup lang="ts">
import gettext from '@/gettext'
import StdDataEntry from '@/components/StdDataEntry'
import StdPagination from './StdPagination.vue'
import {computed, onMounted, reactive, ref, watch} from 'vue'
import {useRoute, useRouter} from 'vue-router'
import {message} from 'ant-design-vue'
import {downloadCsv} from '@/lib/helper'

const {$gettext, interpolate} = gettext

const emit = defineEmits(['onSelected', 'onSelectedRecord', 'clickEdit'])

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
const params = reactive({
    ...props.get_params
})

const selectedRowKeys = ref([])

const searchColumns = getSearchColumns()
const pithyColumns = getPithyColumns()

onMounted(() => {
    if (!props.disable_query_params) {
        Object.assign(params, route.query)
    }
    get_list()
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
    emit('onSelected', selectedRowKeys.value)
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
    router.push({query: params})
    get_list()
})

const rowSelection = computed(() => {
    if (props.selectionType) {
        return {
            selectedRowKeys: selectedRowKeys, onChange: onSelectChange,
            onSelect: onSelect, type: props.selectionType
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
            message.error(e.message ?? '系统错误')
        })
        page += 1
    }
    const data: any[] = []
    dataSource.forEach((row: Object) => {
        let obj: any = {}
        headerKeys.forEach(key => {
            console.log(row, key)
            let data = fn(row, key)
            const c = showColumnsMap[key]
            console.log(c)
            data = c?.customRender?.({text: data}) ?? data
            obj[c.dataIndex] = data
        })
        data.push(obj)
    })
    console.log(header, data)
    downloadCsv(header, data, '测试.csv')
}
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
                <a-space class="reset-btn">
                    <a-button @click="export_csv" type="primary" ghost>
                        <translate>Export</translate>
                    </a-button>
                    <a-button @click="reset_search">
                        <translate>Reset</translate>
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
    align-items: flex-start;
}

:deep(.ant-form-inline .ant-form-item) {
    margin-bottom: 10px;
}
</style>
