<script setup lang="ts">
import {reactive, ref} from 'vue'
import gettext from '@/gettext'
import StdDataEntry from '@/components/StdDataEntry'
import {message} from 'ant-design-vue'

const {$gettext} = gettext

const emit = defineEmits(['onSave'])

const props = defineProps(['api', 'beforeSave'])

const batchColumns = ref([])

const visible = ref(false)

const selectedRowKeys = ref([])

function showModal(c: any, rowKeys: any) {
    visible.value = true
    selectedRowKeys.value = rowKeys
    batchColumns.value = c
}

defineExpose({
    showModal
})

const data = reactive({})
const error = reactive({})
const loading = ref(false)

async function ok() {
    loading.value = true

    await props.beforeSave?.()

    await props.api(selectedRowKeys.value, data).then(async () => {
        message.success($gettext('Save successfully'))
        emit('onSave')
    }).catch((e: any) => {
        message.error($gettext(e?.message) ?? $gettext('Server error'))
    }).finally(() => {
        loading.value = false
    })
}
</script>

<template>
    <a-modal
        class="std-curd-edit-modal"
        :mask="false"
        :title="$gettext('Batch Modify')"
        v-model:visible="visible"
        :cancel-text="$gettext('Cancel')"
        :ok-text="$gettext('OK')"
        @ok="ok"
        :confirm-loading="loading"
        :width="600"
        destroyOnClose
    >

        <std-data-entry
            ref="std_data_entry"
            :data-list="batchColumns"
            :data-source="data"
            :error="error"
        />

        <slot name="extra"/>
    </a-modal>
</template>

<style scoped>

</style>
