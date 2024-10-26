<script setup lang="ts">
import type Curd from '@/api/curd'
import type { Column } from '@/components/StdDesign/types'
import { getPithyColumns } from '@/components/StdDesign/StdDataDisplay/methods/columns'
import StdDataEntry from '@/components/StdDesign/StdDataEntry'
import { message } from 'ant-design-vue'

const props = defineProps<{
  // eslint-disable-next-line ts/no-explicit-any
  api: Curd<any>
  beforeSave?: () => Promise<void>
  columns: Column[]
}>()

const emit = defineEmits(['save'])

const batchColumns = ref<Column[]>([])
const selectedRowKeys = ref<(number | string)[]>([])
// eslint-disable-next-line ts/no-explicit-any
const selectedRows = ref<any[]>([])

const visible = ref(false)
const data = ref({})
const error = ref({})
const loading = ref(false)

// eslint-disable-next-line ts/no-explicit-any
function showModal(c: Column[], rowKeys: (number | string)[], rows: any[]) {
  data.value = {}
  visible.value = true
  selectedRowKeys.value = rowKeys
  batchColumns.value = c
  selectedRows.value = rows
}

defineExpose({
  showModal,
})

async function ok() {
  loading.value = true

  await props.beforeSave?.()

  await props.api.batch_save(selectedRowKeys.value, data.value)
    .then(async () => {
      message.success($gettext('Save successfully'))
      emit('save')
      visible.value = false
    })
    .catch(e => {
      error.value = e.errors
      message.error($gettext(e?.message) ?? $gettext('Server error'))
    })
    .finally(() => {
      loading.value = false
    })
}
</script>

<template>
  <AModal
    v-model:open="visible"
    class="std-curd-edit-modal"
    :mask="false"
    :title="$gettext('Batch Modify')"
    :cancel-text="$gettext('No')"
    :ok-text="$gettext('Save')"
    :confirm-loading="loading"
    :width="600"
    destroy-on-close
    @ok="ok"
  >
    <p>{{ $gettext('Belows are selected items that you want to batch modify') }}</p>
    <ATable
      class="mb-4"
      size="small"
      :columns="getPithyColumns(columns)"
      :data-source="selectedRows"
      :pagination="{ showSizeChanger: false, pageSize: 5, size: 'small' }"
    />

    <p>{{ $gettext('Leave blank if do not want to modify') }}</p>
    <StdDataEntry
      :data-list="batchColumns"
      :data-source="data"
      :errors="error"
    />

    <slot name="extra" />
  </AModal>
</template>

<style scoped></style>
