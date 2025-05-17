<script setup lang="ts" generic="T=any">
import type { ComputedRef } from 'vue'
import type { StdCurdProps, StdTableProps } from '@/components/StdDesign/StdDataDisplay/types'
import type { Column } from '@/components/StdDesign/types'
import { message } from 'ant-design-vue'
import StdBatchEdit from '@/components/StdDesign/StdDataDisplay/StdBatchEdit.vue'
import StdCurdDetail from '@/components/StdDesign/StdDataDisplay/StdCurdDetail.vue'
import StdDataEntry from '@/components/StdDesign/StdDataEntry'
import StdTable from './StdTable.vue'

const props = defineProps<StdTableProps<T> & StdCurdProps<T>>()

const selectedRowKeys = defineModel<(number | string)[]>('selectedRowKeys', {
  default: () => reactive([]),
})

const selectedRows = defineModel<T[]>('selectedRows', {
  default: () => reactive([]),
})

const visible = ref(false)
// eslint-disable-next-line ts/no-explicit-any
const data: any = reactive({ id: null })
const modifyMode = ref(true)
const editMode = ref<string>()
const shouldRefetchList = ref(false)

provide('data', data)
provide('editMode', editMode)
provide('shouldRefetchList', shouldRefetchList)

// eslint-disable-next-line ts/no-explicit-any
const error: any = reactive({})
const selected = ref([])

// eslint-disable-next-line ts/no-explicit-any
function onSelect(keys: any) {
  selected.value = keys
}

const editableColumns = computed(() => {
  return props.columns!.filter(c => {
    return c.edit
  })
}) as ComputedRef<Column[]>

// eslint-disable-next-line ts/no-explicit-any
function add(preset: any = undefined) {
  if (props.onClickAdd)
    return
  Object.keys(data).forEach(v => {
    delete data[v]
  })

  if (preset)
    Object.assign(data, preset)

  clearError()
  visible.value = true
  editMode.value = 'create'
  modifyMode.value = true
}

const table = useTemplateRef('table')
const inTrash = ref(false)
const getParams = reactive(props.getParams ?? {})

function get_list() {
  table.value?.get_list()
}

defineExpose({
  add,
  get_list,
  data,
  inTrash,
})

function clearError() {
  Object.keys(error).forEach(v => {
    delete error[v]
  })
}

// eslint-disable-next-line vue/require-typed-ref
const stdEntryRef = ref()

async function ok() {
  const { formRef } = stdEntryRef.value

  clearError()
  try {
    await formRef.validateFields()
    props?.beforeSave?.(data)
    props
      .api!.save(data.id, { ...data, ...props.overwriteParams }, { params: { ...props.overwriteParams } }).then(r => {
      message.success($gettext('Save successfully'))
      Object.assign(data, r)
      get_list()
      visible.value = false
    }).catch(e => {
      Object.assign(error, e.errors)
    })
  }
  catch {
    message.error($gettext('Please fill in the required fields'))
  }
}

function cancel() {
  visible.value = false

  clearError()

  if (shouldRefetchList.value) {
    get_list()
    shouldRefetchList.value = false
  }
}

function edit(id: number | string) {
  if (props.onClickEdit)
    return
  get(id).then(() => {
    visible.value = true
    modifyMode.value = true
    editMode.value = 'modify'
  })
}

function view(id: number | string) {
  get(id).then(() => {
    visible.value = true
    modifyMode.value = false
  })
}

async function get(id: number | string) {
  return props
    .api!.get(id, { ...props.overwriteParams }).then(async r => {
    Object.keys(data).forEach(k => {
      delete data[k]
    })
    data.id = null
    Object.assign(data, r)
  })
}

const modalTitle = computed(() => {
  // eslint-disable-next-line sonarjs/no-nested-conditional
  return data.id ? modifyMode.value ? $gettext('Modify') : $gettext('View Details') : $gettext('Add')
})

const localOverwriteParams = reactive(props.overwriteParams ?? {})

const stdBatchEditRef = useTemplateRef('stdBatchEditRef')

async function handleClickBatchEdit(batchColumns: Column[]) {
  stdBatchEditRef.value?.showModal(batchColumns, selectedRowKeys.value, selectedRows.value)
}

function handleBatchUpdated() {
  table.value?.get_list()
  table.value?.resetSelection()
}
</script>

<template>
  <div class="std-curd">
    <ACard>
      <template #title>
        <div class="flex items-center">
          {{ title || $gettext('List') }}
          <slot name="title-slot" />
        </div>
      </template>
      <template #extra>
        <ASpace>
          <slot name="beforeAdd" />
          <AButton
            v-if="!disableAdd && !inTrash"
            type="link"
            size="small"
            @click="add()"
          >
            {{ $gettext('Add') }}
          </AButton>
          <slot name="extra" />
          <template v-if="!disableDelete">
            <AButton
              v-if="!inTrash"
              type="link"
              size="small"
              :loading="table?.loading"
              @click="inTrash = true"
            >
              {{ $gettext('Trash') }}
            </AButton>
            <AButton
              v-else
              type="link"
              size="small"
              :loading="table?.loading"
              @click="inTrash = false"
            >
              {{ $gettext('Back to list') }}
            </AButton>
          </template>
        </ASpace>
      </template>

      <slot name="beforeTable" />
      <StdTable
        ref="table"
        v-bind="{
          ...props,
          getParams,
          overwriteParams: localOverwriteParams,
        }"
        v-model:selected-row-keys="selectedRowKeys"
        v-model:selected-rows="selectedRows"
        :in-trash="inTrash"
        @click-edit="edit"
        @click-view="view"
        @selected="onSelect"
        @click-batch-modify="handleClickBatchEdit"
      >
        <template
          v-for="(_, key) in $slots"
          :key="key"
          #[key]="slotProps"
        >
          <slot
            :name="key"
            v-bind="slotProps"
          />
        </template>
      </StdTable>
    </ACard>

    <AModal
      class="std-curd-edit-modal"
      :mask="modalMask"
      :title="modalTitle"
      :open="visible"
      :cancel-text="$gettext('Cancel')"
      :ok-text="$gettext('Ok')"
      :width="modalMaxWidth"
      :footer="modifyMode ? undefined : null"
      destroy-on-close
      @cancel="cancel"
      @ok="ok"
    >
      <div
        v-if="!disableModify && !disableView && editMode === 'modify'"
        class="m-2 flex justify-end"
      >
        <ASwitch
          v-model:checked="modifyMode"
          class="mr-2"
        />
        {{ modifyMode ? $gettext('Modify Mode') : $gettext('View Mode') }}
      </div>

      <template v-if="modifyMode">
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
          ref="stdEntryRef"
          :data-list="editableColumns"
          :data-source="data"
          :errors="error"
        />

        <slot
          name="edit"
          :data="data"
        />
      </template>

      <StdCurdDetail
        v-else
        :columns
        :data
      />
    </AModal>

    <StdBatchEdit
      ref="stdBatchEditRef"
      :api
      :columns
      @save="handleBatchUpdated"
    />
  </div>
</template>

<style lang="less" scoped>
:deep(.before-edit:last-child) {
  margin-bottom: 20px;
}
</style>
