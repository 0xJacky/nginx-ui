<script setup lang="ts" generic="T=any">
import type { GetListResponse, Pagination } from '@/api/curd'
import type { StdTableProps } from '@/components/StdDesign/StdDataDisplay/types'
import type { Column } from '@/components/StdDesign/types'
import type { TableProps } from 'ant-design-vue'
import type { Key } from 'ant-design-vue/es/_util/type'
import type { FilterValue } from 'ant-design-vue/es/table/interface'
import type { SorterResult, TablePaginationConfig } from 'ant-design-vue/lib/table/interface'
import type { ComputedRef, Ref } from 'vue'
import type { RouteParams } from 'vue-router'
import { getPithyColumns } from '@/components/StdDesign/StdDataDisplay/methods/columns'
import useSortable from '@/components/StdDesign/StdDataDisplay/methods/sortable'
import StdBulkActions from '@/components/StdDesign/StdDataDisplay/StdBulkActions.vue'
import StdDataEntry, { labelRender } from '@/components/StdDesign/StdDataEntry'
import { HolderOutlined } from '@ant-design/icons-vue'
import { message } from 'ant-design-vue'
import _ from 'lodash'
import StdPagination from './StdPagination.vue'

const props = withDefaults(defineProps<StdTableProps<T>>(), {
  rowKey: 'id',
})

const emit = defineEmits([
  'clickEdit',
  'clickView',
  'clickBatchModify',
])

const selectedRowKeys = defineModel<(number | string)[]>('selectedRowKeys', {
  default: () => reactive([]),
})

const selectedRows = defineModel<T[]>('selectedRows', {
  default: () => reactive([]),
})

const route = useRoute()

const dataSource: Ref<T[]> = ref([])
const expandKeysList: Ref<Key[]> = ref([])

watch(dataSource, () => {
  if (!props.expandAll)
    return

  const res: Key[] = []

  function buildKeysList(record) {
    record.children?.forEach(v => {
      buildKeysList(v)
    })
    res.push(record[props.rowKey])
  }

  dataSource.value.forEach(v => {
    buildKeysList(v)
  })

  expandKeysList.value = res
})

// eslint-disable-next-line ts/no-explicit-any
const rowsKeyIndexMap: Ref<Record<number, any>> = ref({})
const loading = ref(true)
// eslint-disable-next-line ts/no-explicit-any
const selectedRecords: Ref<Record<any, any>> = ref({})

// This can be useful if there are more than one StdTable in the same page.
// eslint-disable-next-line sonarjs/pseudo-random
const randomId = ref(Math.random().toString(36).substring(2, 8))
const updateFilter = ref(0)
const init = ref(false)

const pagination: Pagination = reactive({
  total: 1,
  per_page: 10,
  current_page: 1,
  total_pages: 1,
})

const filterParams = ref({})

const paginationParams = ref({
  page: 1,
  page_size: 20,
})

const sortParams = ref({
  order: 'desc' as 'desc' | 'asc' | undefined,
  sort_by: '' as Key | readonly Key[] | undefined,
})

const params = computed(() => {
  return {
    ...filterParams.value,
    ...sortParams.value,
    ...props.getParams,
    ...props.overwriteParams,
    trash: props.inTrash,
  }
})

onMounted(() => {
  selectedRows.value.forEach(v => {
    selectedRecords.value[v[props.rowKey]] = v
  })
})

const searchColumns = computed(() => {
  const _searchColumns: Column[] = []

  props.columns.forEach((column: Column) => {
    if (column.search) {
      if (typeof column.search === 'object') {
        _searchColumns.push({
          ...column,
          edit: column.search,
        })
      }

      else {
        _searchColumns.push({ ...column })
      }
    }
  })

  return _searchColumns
})

const pithyColumns = computed<Column[]>(() => {
  if (props.pithy)
    return getPithyColumns(props.columns)

  return props.columns?.filter(c => {
    return !c.hiddenInTable
  })
})

const batchColumns = computed(() => {
  return props.columns?.filter(column => column.batch) || []
})

const radioColumns = computed(() => {
  return props.columns?.filter(column => column.radio) || []
})

const get_list = _.debounce(_get_list, 100, {
  leading: false,
  trailing: true,
})

onMounted(async () => {
  if (!props.disableQueryParams) {
    filterParams.value = {
      ...route.query,
      ...props.getParams,
    }
    paginationParams.value.page = Number(route.query.page) || 1
    paginationParams.value.page_size = Number(route.query.page_size) || 20
  }

  await nextTick()

  get_list()

  if (props.sortable)
    initSortable()

  init.value = true
})

defineExpose({
  get_list,
  pagination,
  resetSelection,
  loading,
})

function destroy(id: number | string) {
  props.api!.destroy(id, { permanent: props.inTrash }).then(() => {
    get_list()
    message.success($gettext('Deleted successfully'))
  }).catch(e => {
    message.error($gettext(e?.message ?? 'Server error'))
  })
}

function recover(id: number | string) {
  props.api.recover(id).then(() => {
    message.success($gettext('Recovered Successfully'))
    get_list()
  }).catch(e => {
    message.error(e?.message ?? $gettext('Server error'))
  })
}

// eslint-disable-next-line ts/no-explicit-any
function buildIndexMap(data: any, level: number = 0, index: number = 0, total: number[] = []) {
  if (data && data.length > 0) {
  // eslint-disable-next-line ts/no-explicit-any
    data.forEach((v: any) => {
      v.level = level

      const current_indexes = [...total, index++]

      rowsKeyIndexMap.value[v.id] = current_indexes
      if (v.children)
        buildIndexMap(v.children, level + 1, 0, current_indexes)
    })
  }
}

async function _get_list() {
  dataSource.value = []
  loading.value = true

  // eslint-disable-next-line ts/no-explicit-any
  await props.api?.get_list({ ...params.value, ...paginationParams.value }).then(async (r: GetListResponse<any>) => {
    dataSource.value = r.data
    rowsKeyIndexMap.value = {}
    if (props.sortable)
      buildIndexMap(r.data)

    if (r.pagination)
      Object.assign(pagination, r.pagination)
  }).catch(e => {
    message.error($gettext(e?.message ?? 'Server error'))
  })

  loading.value = false
}

// eslint-disable-next-line ts/no-explicit-any
function onTableChange(_pagination: TablePaginationConfig, filters: Record<string, FilterValue>, sorter: SorterResult | SorterResult<any>[]) {
  if (sorter) {
    sorter = sorter as SorterResult
    selectedRowKeys.value = []
    sortParams.value.sort_by = sorter.field
    switch (sorter.order) {
      case 'ascend':
        sortParams.value.order = 'asc'
        break
      case 'descend':
        sortParams.value.order = 'desc'
        break
      default:
        sortParams.value.order = undefined
        break
    }
  }
  if (filters) {
    Object.keys(filters).forEach((v: string) => {
      params[v] = filters[v]
    })
  }

  if (_pagination)
    selectedRowKeys.value = []
}

function expandedTable(keys: Key[]) {
  expandKeysList.value = keys
}

// eslint-disable-next-line ts/no-explicit-any
async function onSelect(record: any, selected: boolean, _selectedRows: any[]) {
  // console.log('onSelect', record, selected, _selectedRows)
  if (props.selectionType === 'checkbox' || props.exportExcel || batchColumns.value.length > 0 || props.bulkActions) {
    if (selected) {
      _selectedRows.forEach(v => {
        if (v) {
          if (selectedRecords.value[v[props.rowKey]] === undefined)
            selectedRowKeys.value.push(v[props.rowKey])

          selectedRecords.value[v[props.rowKey]] = v
        }
      })
    }
    else {
      selectedRowKeys.value.splice(selectedRowKeys.value.indexOf(record[props.rowKey]), 1)
      delete selectedRecords.value[record[props.rowKey]]
    }
    await nextTick()
    selectedRows.value = [...selectedRowKeys.value.map(v => selectedRecords.value[v])]
  }
  else if (selected) {
    selectedRowKeys.value = record[props.rowKey]
    selectedRows.value = [record]
  }
  else {
    selectedRowKeys.value = []
    selectedRows.value = []
  }
}

// eslint-disable-next-line ts/no-explicit-any
async function onSelectAll(selected: boolean, _selectedRows: any[], changeRows: any[]) {
  // console.log('onSelectAll', selected, selectedRows, changeRows)
  // eslint-disable-next-line ts/no-explicit-any
  changeRows.forEach((v: any) => {
    if (v) {
      if (selected) {
        selectedRowKeys.value.push(v[props.rowKey])
        selectedRecords.value[v[props.rowKey]] = v
      }
      else {
        delete selectedRecords.value[v[props.rowKey]]
      }
    }
  })

  if (!selected) {
    selectedRowKeys.value.splice(0, selectedRowKeys.value.length, ...selectedRowKeys.value.filter(v => selectedRecords.value[v]))
  }

  // console.log(selectedRowKeysBuffer.value, selectedRecords.value)

  await nextTick()
  selectedRows.value.splice(0, selectedRows.value.length, ...selectedRowKeys.value.map(v => selectedRecords.value[v]))
}

function resetSelection() {
  selectedRowKeys.value = reactive([])
  selectedRows.value = reactive([])
  selectedRecords.value = reactive({})
}

const router = useRouter()

async function resetSearch() {
  filterParams.value = {}
  updateFilter.value++
}

watch(params, async v => {
  if (!init.value)
    return

  paginationParams.value = {
    page: 1,
    page_size: paginationParams.value.page_size,
  }

  await nextTick()

  if (!props.disableQueryParams)
    await router.push({ query: { ...v as unknown as RouteParams, ...paginationParams.value } })
  else
    get_list()
}, { deep: true })

watch(() => route.query, () => {
  if (init.value)
    get_list()
})

const rowSelection = computed(() => {
  if (batchColumns.value.length > 0 || props.selectionType || props.exportExcel || props.bulkActions) {
    return {
      selectedRowKeys: unref(selectedRowKeys),
      onSelect,
      onSelectAll,
      getCheckboxProps: props?.getCheckboxProps,
      type: (batchColumns.value.length > 0 || props.exportExcel || props.bulkActions) ? 'checkbox' : props.selectionType,
    }
  }
  else {
    return null
  }
}) as ComputedRef<TableProps['rowSelection']>

const hasSelectedRow = computed(() => {
  return batchColumns.value.length > 0 && selectedRowKeys.value.length > 0
})

function clickBatchEdit() {
  emit('clickBatchModify', batchColumns.value, selectedRowKeys.value, selectedRows.value)
}

function initSortable() {
  useSortable(props, randomId, dataSource, rowsKeyIndexMap, expandKeysList)
}

async function changePage(page: number, page_size: number) {
  if (page) {
    paginationParams.value = {
      page,
      page_size,
    }
  }
  else {
    paginationParams.value = {
      page: 1,
      page_size,
    }
  }

  await nextTick()

  if (!props.disableQueryParams)
    await router.push({ query: { ...route.query, ...paginationParams.value } })

  get_list()
}

const paginationSize = computed(() => {
  if (props.size === 'small')
    return 'small'
  else
    return 'default'
})
</script>

<template>
  <div class="std-table">
    <div v-if="radioColumns.length">
      <AFormItem
        v-for="column in radioColumns"
        :key="column.dataIndex as PropertyKey"
        :label="labelRender(column.title)"
      >
        <ARadioGroup v-model:value="params[column.dataIndex as string]">
          <ARadioButton :value="undefined">
            {{ $gettext('All') }}
          </ARadioButton>
          <ARadioButton
            v-for="(value, key) in column.mask"
            :key
            :value="key"
          >
            {{ labelRender(value) }}
          </ARadioButton>
        </ARadioGroup>
      </AFormItem>
    </div>
    <StdDataEntry
      v-if="!disableSearch && searchColumns.length"
      :key="updateFilter"
      :data-list="searchColumns"
      :data-source="filterParams"
      type="search"
      layout="inline"
    >
      <template #action>
        <ASpace class="action-btn">
          <AButton @click="resetSearch">
            {{ $gettext('Reset') }}
          </AButton>
          <AButton
            v-if="hasSelectedRow"
            @click="clickBatchEdit"
          >
            {{ $gettext('Batch Modify') }}
          </AButton>
          <slot name="append-search" />
        </ASpace>
      </template>
    </StdDataEntry>
    <StdBulkActions
      v-if="bulkActions"
      v-model:selected-row-keys="selectedRowKeys"
      :api
      :in-trash="inTrash"
      :actions="bulkActions"
      @on-success="() => { resetSelection(); get_list() }"
    />
    <ATable
      :id="`std-table-${randomId}`"
      :columns="pithyColumns"
      :data-source="dataSource"
      :loading="loading"
      :pagination="false"
      :row-key="rowKey"
      :row-selection="rowSelection"
      :scroll="{ x: scrollX ?? true }"
      :size="size as any"
      :expanded-row-keys="expandKeysList"
      @change="onTableChange"
      @expanded-rows-change="expandedTable"
    >
      <template #bodyCell="{ text, record, column }: {text: any, record: Record<string, any>, column: any}">
        <template v-if="column.handle === true">
          <span class="ant-table-drag-icon"><HolderOutlined /></span>
          {{ text }}
        </template>
        <div v-if="column.dataIndex === 'action'" class="action">
          <template v-if="!props.disableView && !inTrash">
            <AButton
              type="link"
              size="small"
              @click="$emit('clickView', record[props.rowKey], record)"
            >
              {{ $gettext('View') }}
            </AButton>
          </template>

          <template v-if="!props.disableModify && !inTrash">
            <AButton
              type="link"
              size="small"
              @click="$emit('clickEdit', record[props.rowKey], record)"
            >
              {{ $gettext('Modify') }}
            </AButton>
          </template>

          <slot
            name="actions"
            :record="record"
          />

          <template v-if="!props.disableDelete">
            <APopconfirm
              v-if="!inTrash"
              :cancel-text="$gettext('No')"
              :ok-text="$gettext('Ok')"
              :title="$gettext('Are you sure you want to delete this item?')"
              @confirm="destroy(record[rowKey])"
            >
              <AButton
                type="link"
                size="small"
              >
                {{ $gettext('Delete') }}
              </AButton>
            </APopconfirm>
            <APopconfirm
              v-else
              :cancel-text="$gettext('No')"
              :ok-text="$gettext('Ok')"
              :title="$gettext('Are you sure you want to recover this item?')"
              @confirm="recover(record[rowKey])"
            >
              <AButton
                type="link"
                size="small"
              >
                {{ $gettext('Recover') }}
              </AButton>
            </APopconfirm>
            <APopconfirm
              v-if="inTrash"
              :cancel-text="$gettext('No')"
              :ok-text="$gettext('Ok')"
              :title="$gettext('Are you sure you want to delete this item permanently?')"
              @confirm="destroy(record[rowKey])"
            >
              <AButton
                type="link"
                size="small"
              >
                {{ $gettext('Delete Permanently') }}
              </AButton>
            </APopconfirm>
          </template>
        </div>
      </template>
    </ATable>
    <StdPagination
      :size="paginationSize"
      :loading="loading"
      :pagination="pagination"
      @change="changePage"
      @change-page-size="onTableChange"
    />
  </div>
</template>

<style lang="less">
.ant-table-scroll {
  .ant-table-body {
    overflow-x: auto !important;
    overflow-y: hidden !important;
  }
}

.std-table {
  overflow-x: hidden !important;
  overflow-y: hidden !important;
}
</style>

<style lang="less" scoped>
.ant-form {
  margin: 10px 0 20px 0;
}

.ant-slider {
  min-width: 90px;
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

.ant-divider {
  &:last-child {
    display: none;
  }
}

.action {
  @media (max-width: 768px) {
    .ant-divider-vertical {
      display: none;
    }
  }
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
