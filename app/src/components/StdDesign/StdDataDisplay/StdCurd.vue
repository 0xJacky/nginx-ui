<script setup lang="ts">
import { message } from 'ant-design-vue'
import type { ComputedRef } from 'vue'
import type { StdTableProps } from './StdTable.vue'
import StdTable from './StdTable.vue'
import StdDataEntry from '@/components/StdDesign/StdDataEntry'
import type { Column } from '@/components/StdDesign/types'
import StdCurdDetail from '@/components/StdDesign/StdDataDisplay/StdCurdDetail.vue'

export interface StdCurdProps {
  cardTitleKey?: string
  modalMaxWidth?: string | number
  modalMask?: boolean
  exportExcel?: boolean
  importExcel?: boolean

  disableAdd?: boolean
  onClickAdd?: () => void
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  onClickEdit?: (id: number | string, record: any, index: number) => void
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  beforeSave?: (data: any) => Promise<void>
}

const props = defineProps<StdTableProps & StdCurdProps>()
const route = useRoute()
const router = useRouter()
const visible = ref(false)
const update = ref(0)
// eslint-disable-next-line @typescript-eslint/no-explicit-any
const data: any = reactive({ id: null })
const modifyMode = ref(true)
const editMode = ref<string>()

provide('data', data)
provide('editMode', editMode)

// eslint-disable-next-line @typescript-eslint/no-explicit-any
const error: any = reactive({})
const selected = ref([])
// eslint-disable-next-line @typescript-eslint/no-explicit-any
function onSelect(keys: any) {
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
  editMode.value = 'create'
  modifyMode.value = true
}
const table = ref()

const selectedRowKeys = ref([])

const getParams = reactive({
  trash: false,
})

// eslint-disable-next-line @typescript-eslint/no-explicit-any
const setParams = (k: string, v: any) => {
  getParams[k] = v
}

function get_list() {
  table.value?.get_list()
}

defineExpose({
  add,
  get_list,
  data,
  getParams,
  setParams,
})

function clear_error() {
  Object.keys(error).forEach(v => {
    delete error[v]
  })
}

const stdEntryRef = ref()
async function ok() {
  const { formRef } = stdEntryRef.value

  clear_error()
  try {
    await formRef.validateFields()
    await props?.beforeSave?.(data)
    props
      .api!.save(data.id, { ...data, ...props.overwriteParams }, { params: { ...props.overwriteParams } })
      .then(r => {
        message.success($gettext('Save successfully'))
        Object.assign(data, r)
        get_list()
        visible.value = false
      })
      .catch(e => {
        message.error($gettext(e?.message ?? 'Server error'), 5)
        Object.assign(error, e.errors)
      })
  }
  catch { }
}

function cancel() {
  visible.value = false

  clear_error()
}

watch(visible, v => {
  if (!v) {
    router.push({
      query: {
        ...route.query,
        [`open.${props.rowKey || 'id'}`]: undefined,
      },
    })
  }
})

watch(modifyMode, v => {
  router.push({
    query: {
      ...route.query,
      modify_mode: v.toString(),
    },
  })
})

function edit(id: number | string) {
  get(id, true).then(() => {
    visible.value = true
    modifyMode.value = true
    editMode.value = 'modify'
  }).catch(e => {
    message.error($gettext(e?.message ?? 'Server error'), 5)
  })
}

function view(id: number | string) {
  get(id, false).then(() => {
    visible.value = true
    modifyMode.value = false
  }).catch(e => {
    message.error($gettext(e?.message ?? 'Server error'), 5)
  })
}

async function get(id: number | string, _modifyMode: boolean) {
  return props
    .api!.get(id, { ...props.overwriteParams })
    .then(async r => {
      Object.keys(data).forEach(k => {
        delete data[k]
      })
      data.id = null
      Object.assign(data, r)
      if (!props.disabledModify) {
        await router.push({
          query: {
            ...route.query,
            [`open.${props.rowKey || 'id'}`]: id,
            modify_mode: _modifyMode.toString(),
          },
        })
      }
    })
}

onMounted(async () => {
  const id = route.query[`open.${props.rowKey || 'id'}`] as string
  const _modifyMode = (route.query.modify_mode as string) === 'true'
  if (id && !props.disabledModify && _modifyMode)
    edit(id)

  if (id && !props.disabledView && !_modifyMode)
    view(id)
})

const modalTitle = computed(() => {
  return data.id ? modifyMode.value ? $gettext('Modify') : $gettext('View Details') : $gettext('Add')
})

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
          <a
            v-if="!disableAdd"
            @click="add"
          >{{ $gettext('Add') }}</a>
          <slot name="extra" />
          <template v-if="!disableDelete">
            <a
              v-if="!getParams.trash"
              @click="getParams.trash = true"
            >
              {{ $gettext('Trash') }}
            </a>
            <a
              v-else
              @click="getParams.trash = false"
            >
              {{ $gettext('List') }}
            </a>
          </template>
        </ASpace>
      </template>

      <StdTable
        ref="table"
        :key="update"
        v-model:selected-row-keys="selectedRowKeys"
        v-bind="{
          ...props,
          getParams,
        }"
        @click-edit="edit"
        @click-view="view"
        @selected="onSelect"
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
        v-if="!disabledModify && !disabledView"
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
          :error="error"
        />

        <slot
          name="edit"
          :data="data"
        />
      </template>

      <StdCurdDetail
        v-else
        :columns="columns"
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
