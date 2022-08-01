<script setup lang="ts">
import gettext from '@/gettext'

const {$gettext, interpolate} = gettext
import StdTable from './StdTable.vue'

import StdDataEntry from '@/components/StdDataEntry'

import {reactive, ref} from 'vue'
import {message} from 'ant-design-vue'

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
})

const visible = ref(false)
const update = ref(0)
let data = reactive({id: null})
let error = reactive({})
const params = reactive({})
const selected = reactive([])

function onSelect(keys: any) {
    selected.concat(...keys)
}

function editableColumns() {
    return props.columns!.filter((c: any) => {
        return c.edit
    })
}

function add() {
    data = reactive({
        id: null
    })
    visible.value = true
}

const table = ref(null)

interface Table {
    get_list(): void
}

const ok = async () => {
    error = reactive({})
    props.api!.save(data.id, data).then((r: any) => {
        message.success($gettext('Save Successfully'))
        Object.assign(data, r)
        const t: Table | null = table.value
        t!.get_list()

    }).catch((e: any) => {
        message.error((e?.message ?? $gettext('Server error')), 5)
        error = e.errors
    })
}

function cancel() {
    visible.value = false
    error = reactive({})
}

function edit(id: any) {
    props.api!.get(id).then((r: any) => {
        Object.assign(data, r)
        visible.value = true
    }).catch((e: any) => {
        message.error((e?.message ?? $gettext('Server error')), 5)
    })
}

</script>

<template>
    <div class="std-curd">
        <a-card :title="title||$gettext('Table')">
            <template v-if="!disable_add" #extra>
                <a @click="add" v-translate>Add</a>
            </template>

            <std-table
                ref="table"
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
            :title="data.id ? $gettext('Modify') : $gettext('Add')"
            :visible="visible"
            :cancel-text="$gettext('Cancel')"
            :ok-text="$gettext('OK')"
            @cancel="cancel"
            @ok="ok"
            :width="600"
            destroyOnClose
        >
            <std-data-entry
                ref="std_data_entry"
                :data-list="editableColumns()"
                :data-source="data"
                :error="error"
            />
        </a-modal>
    </div>
</template>

<style lang="less" scoped>

</style>
