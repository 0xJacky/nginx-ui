<script setup lang="ts">
import { message } from 'ant-design-vue'
import type { ComputedRef } from 'vue'
import type { StdTableProps } from './StdTable.vue'
import StdTable from './StdTable.vue'
import gettext from '@/gettext'
import StdDataEntry from '@/components/StdDesign/StdDataEntry'
import type { Column } from '@/components/StdDesign/types'

export interface StdCurdProps {
  cardTitleKey?: string
  modalMaxWidth?: string | number
  disableAdd?: boolean
  onClickAdd?: () => void
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  onClickEdit?: (id: number | string, record: any, index: number) => void
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  beforeSave?: (data: any) => void
}

const props = defineProps<StdTableProps & StdCurdProps>()

const { $gettext } = gettext

const visible = ref(false)
const update = ref(0)
const data = reactive({ id: null })

provide('data', data)

const error = reactive({})
const selected = ref([])

function onSelect(keys) {
  selected.value = keys
}

const editableColumns = computed(() => {
  return props.columns!.filter(c => {
    return c.edit
  })
}) as ComputedRef<Column[]>

function add() {
  Object.keys(data).forEach(v => {
    delete data[v]
  })

  clear_error()
  visible.value = true
}
const table = ref()
function get_list() {
  table.value?.get_list()
}

defineExpose({
  add,
  get_list,
  data,
})

function clear_error() {
  Object.keys(error).forEach(v => {
    delete error[v]
  })
}

const ok = async () => {
  clear_error()
  await props?.beforeSave?.(data)
  props.api!.save(data.id, data).then(r => {
    message.success($gettext('Save Successfully'))
    Object.assign(data, r)
    get_list()
    visible.value = false
  }).catch(e => {
    message.error($gettext(e?.message ?? 'Server error'), 5)
    Object.assign(error, e.errors)
  })
}

function cancel() {
  visible.value = false

  clear_error()
}

function edit(id) {
  props.api!.get(id).then(async r => {
    Object.keys(data).forEach(k => {
      delete data[k]
    })
    data.id = null
    Object.assign(data, r)
    visible.value = true
  }).catch(e => {
    message.error($gettext(e?.message ?? 'Server error'), 5)
  })
}

const selectedRowKeys = ref([])
</script>

<template>
  <div class="std-curd">
    <ACard :title="title || $gettext('Table')">
      <template
        v-if="!disableAdd"
        #extra
      >
        <a @click="add">{{ $gettext('Add') }}</a>
      </template>

      <StdTable
        ref="table"
        v-bind="props"
        :key="update"
        v-model:selected-row-keys="selectedRowKeys"
        @click-edit="edit"
        @selected="onSelect"
      >
        <template #actions="slotProps">
          <slot
            name="actions"
            :actions="slotProps.record"
          />
        </template>
      </StdTable>
    </ACard>

    <AModal
      class="std-curd-edit-modal"
      :mask="false"
      :title="data.id ? $gettext('Modify') : $gettext('Add')"
      :open="visible"
      :cancel-text="$gettext('Cancel')"
      :ok-text="$gettext('OK')"
      :width="modalMaxWidth"
      destroy-on-close
      @cancel="cancel"
      @ok="ok"
    >
      <div
        v-if="$slots.beforeEdit"
        class="before-edit"
      >
        <slot
          name="beforeEdit"
          :data="data"
        />
      </div>

      <StdDataEntry
        :data-list="editableColumns"
        :data-source="data"
        :error="error"
      />

      <slot
        name="edit"
        :data="data"
      />
    </AModal>
  </div>
</template>

<style lang="less" scoped>
:deep(.before-edit:last-child) {
  margin-bottom: 20px;
}
</style>
