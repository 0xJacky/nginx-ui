<script setup lang="ts">
import gettext from '@/gettext'
import StdTable from './StdTable.vue'

import StdDataEntry from '@/components/StdDataEntry'

import {provide, reactive, ref} from 'vue'
import {message} from 'ant-design-vue'

const {$gettext} = gettext

const props = defineProps({
    api: Object,
    columns: Array,
    title: String,
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
    soft_delete: {
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
    beforeSave: {
        type: Function,
        default: () => {
        }
    },
    exportCsv: {
        type: Boolean,
        default: false
    },
    modalWidth: {
        type: Number,
        default: 600
    },
    useSortable: Boolean
})

const visible = ref(false)
const update = ref(0)
const data: any = reactive({id: null})
provide('data', data)
const error: any = reactive({})
const selected = ref([])

function onSelect(keys: any) {
    selected.value = keys
}

function editableColumns() {
    return props.columns!.filter((c: any) => {
        return c.edit
    })
}

function add() {
    Object.keys(data).forEach(v => {
        delete data[v]
    })

    clear_error()
    visible.value = true
}

function get_list() {
    const t: Table = table.value!
    t!.get_list()
}

defineExpose({
    add,
    get_list,
    data
})

const table = ref(null)

interface Table {
    get_list(): void
}

function clear_error() {
    Object.keys(error).forEach(v => {
        delete error[v]
    })
}

const ok = async () => {
    clear_error()
    await props?.beforeSave!?.(data)
    props.api!.save(data.id, data).then((r: any) => {
        message.success($gettext('Save Successfully'))
        Object.assign(data, r)
        get_list()

    }).catch((e: any) => {
        message.error($gettext(e?.message ?? 'Server error'), 5)
        Object.assign(error, e.errors)
    })
}

function cancel() {
    visible.value = false

    clear_error()
}

function edit(id: any) {
    props.api!.get(id).then(async (r: any) => {
        Object.keys(data).forEach(k => {
            delete data[k]
        })
        data.id = null
        Object.assign(data, r)
        visible.value = true
    }).catch((e: any) => {
        message.error($gettext(e?.message ?? 'Server error'), 5)
    })
}

const selectedRowKeys = ref([])
</script>

<template>
    <div class="std-curd">
        <a-card :title="title||$gettext('Table')">
            <template v-if="!disable_add" #extra>
                <a @click="add">{{ $gettext('Add') }}</a>
            </template>

            <std-table
                ref="table"
                v-model:selected-row-keys="selectedRowKeys"
                v-bind="props"
                @clickEdit="edit"
                @selected="onSelect"
                :key="update"
            >
                <template v-slot:actions="slotProps">
                    <slot name="actions" :actions="slotProps.record"/>
                </template>
            </std-table>
        </a-card>

        <a-modal
            class="std-curd-edit-modal"
            :mask="false"
            :title="edit_text?edit_text:(data.id ? $gettext('Modify') : $gettext('Add'))"
            :visible="visible"
            :cancel-text="$gettext('Cancel')"
            :ok-text="$gettext('OK')"
            @cancel="cancel"
            @ok="ok"
            :width="modalWidth"
            destroyOnClose
        >
            <div class="before-edit" v-if="$slots.beforeEdit">
                <slot name="beforeEdit" :data="data"/>
            </div>

            <std-data-entry
                ref="std_data_entry"
                :data-list="editableColumns()"
                :data-source="data"
                :error="error"
            />

            <slot name="edit" :data="data"/>
        </a-modal>
    </div>
</template>

<style lang="less" scoped>
:deep(.before-edit:last-child) {
    margin-bottom: 20px;
}
</style>
