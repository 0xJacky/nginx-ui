<script setup lang="ts" generic="T=any">
import type { Pagination } from '@/api/curd'
import type Curd from '@/api/curd'
import type { Column } from '@/components/StdDesign/types'
import type { TableProps } from 'ant-design-vue'
import type { Key } from 'ant-design-vue/es/_util/type'
import type { FilterValue } from 'ant-design-vue/es/table/interface'
import type { SorterResult, TablePaginationConfig } from 'ant-design-vue/lib/table/interface'
import type { ComputedRef, Ref } from 'vue'
import type { RouteParams } from 'vue-router'
import useSortable from '@/components/StdDesign/StdDataDisplay/methods/sortable'
import StdDataEntry from '@/components/StdDesign/StdDataEntry'
import { HolderOutlined } from '@ant-design/icons-vue'
import { message } from 'ant-design-vue'
import _ from 'lodash'
import StdPagination from './StdPagination.vue'

// eslint-disable-next-line ts/no-explicit-any
export interface StdTableProps<T = any> {
  title?: string
  mode?: string
  rowKey?: string
  api: Curd<T>
  columns: Column[]
  // eslint-disable-next-line ts/no-explicit-any
  getParams?: Record<string, any>
  size?: string
  disableQueryParams?: boolean
  disableSearch?: boolean
  pithy?: boolean
  exportExcel?: boolean
  exportMaterial?: boolean
  // eslint-disable-next-line ts/no-explicit-any
  overwriteParams?: Record<string, any>
  disableView?: boolean
  disableModify?: boolean
  selectionType?: string
  sortable?: boolean
  disableDelete?: boolean
  disablePagination?: boolean
  sortableMoveHook?: (oldRow: number[], newRow: number[]) => boolean
  scrollX?: string | number
  // eslint-disable-next-line ts/no-explicit-any
  getCheckboxProps?: (record: any) => any
}

const props = withDefaults(defineProps<StdTableProps<T>>(), {
  rowKey: 'id',
})

const emit = defineEmits(['clickEdit', 'clickView', 'clickBatchModify', 'update:selectedRowKeys'])
const route = useRoute()

const dataSource: Ref<T[]> = ref([])
const expandKeysList: Ref<Key[]> = ref([])

watch(dataSource, () => {
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
const randomId = ref(Math.random().toString(36).substring(2, 8))
const updateFilter = ref(0)
const init = ref(false)

const pagination: Pagination = reactive({
  total: 1,
  per_page: 10,
  current_page: 1,
  total_pages: 1,
})

const params = reactive({
  ...props.getParams,
})

// eslint-disable-next-line ts/no-explicit-any
const selectedRowKeys = defineModel<any[]>('selectedRowKeys', {
  default: () => [],
})

// eslint-disable-next-line ts/no-explicit-any
const selectedRows = defineModel<any[]>('selectedRows', {
  type: Array,
  default: () => [],
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

      else { _searchColumns.push({ ...column }) }
    }
  })

  return _searchColumns
})

const pithyColumns = computed<Column[]>(() => {
  if (props.pithy) {
    return props.columns?.filter(c => {
      return c.pithy === true && !c.hiddenInTable
    })
  }

  return props.columns?.filter(c => {
    return !c.hiddenInTable
  })
})

const batchColumns = computed(() => {
  const batch: Column[] = []

  props.columns?.forEach(column => {
    if (column.batch)
      batch.push(column)
  })

  return batch
})

const get_list = _.debounce(_get_list, 100, {
  leading: true,
  trailing: false,
})

const filterParams = reactive({})

watch(filterParams, () => {
  Object.assign(params, {
    ...filterParams,
    page: 1,
    trash: route.query.trash === 'true',
  })
})

onMounted(() => {
  if (!props.disableQueryParams) {
    Object.assign(params, {
      ...route.query,
      trash: route.query.trash === 'true',
    })

    Object.assign(filterParams, {
      ...route.query,
    })
  }

  get_list()

  if (props.sortable)
    initSortable()

  if (!selectedRowKeys.value?.length)
    selectedRowKeys.value = []

  init.value = true
})

defineExpose({
  get_list,
  pagination,
})

function destroy(id: number | string) {
  props.api!.destroy(id, { permanent: params.trash }).then(() => {
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

async function _get_list(page_num = null, page_size = 20) {
  dataSource.value = []
  loading.value = true
  if (page_num) {
    params.page = page_num
    params.page_size = page_size
  }
  props.api?.get_list({ ...params, ...props.overwriteParams }).then(async r => {
    dataSource.value = r.data
    rowsKeyIndexMap.value = {}
    if (props.sortable)
      buildIndexMap(r.data)

    if (r.pagination)
      Object.assign(pagination, r.pagination)

    setTimeout(() => {
      loading.value = false
    }, 200)
  }).catch(e => {
    message.error(e?.message ?? $gettext('Server error'))
  })
}

// eslint-disable-next-line ts/no-explicit-any
function onTableChange(_pagination: TablePaginationConfig, filters: Record<string, FilterValue>, sorter: SorterResult | SorterResult<any>[]) {
  if (sorter) {
    sorter = sorter as SorterResult
    selectedRowKeys.value = []
    params.sort_by = sorter.field
    params.order = sorter.order === 'ascend' ? 'asc' : 'desc'
    switch (sorter.order) {
      case 'ascend':
        params.sort = 'asc'
        break
      case 'descend':
        params.sort = 'desc'
        break
      default:
        params.sort = null
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
  if (props.selectionType === 'checkbox' || props.exportExcel) {
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
      // eslint-disable-next-line ts/no-explicit-any
      selectedRowKeys.value = selectedRowKeys.value.filter((v: any) => v !== record[props.rowKey])
      delete selectedRecords.value[record[props.rowKey]]
    }

    await nextTick(async () => {
      // eslint-disable-next-line ts/no-explicit-any
      const filteredRows: any[] = []

      selectedRowKeys.value.forEach(v => {
        filteredRows.push(selectedRecords.value[v])
      })
      selectedRows.value = filteredRows
    })
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
  // console.log(selected, selectedRows, changeRows)
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
    selectedRowKeys.value = selectedRowKeys.value.filter(v => {
      return selectedRecords.value[v]
    })
  }

  // console.log(selectedRowKeysBuffer.value, selectedRecords.value)

  await nextTick(async () => {
    // eslint-disable-next-line ts/no-explicit-any
    const filteredRows: any[] = []

    selectedRowKeys.value.forEach(v => {
      filteredRows.push(selectedRecords.value[v])
    })
    selectedRows.value = filteredRows
  })
}

const router = useRouter()

async function resetSearch() {
  Object.keys(params).forEach(v => {
    delete params[v]
  })

  Object.assign(params, {
    ...props.getParams,
  })

  router.push({ query: {} }).catch(() => {
  })

  Object.keys(filterParams).forEach(v => {
    delete filterParams[v]
  })

  updateFilter.value++
}

watch(params, v => {
  if (!init.value)
    return

  if (!props.disableQueryParams)
    router.push({ query: { ...v as RouteParams } })
  else
    get_list()
})

watch(() => route.query, async () => {
  params.trash = route.query.trash === 'true'
  params.team_id = route.query.team_id

  if (init.value)
    await get_list()
})

if (props.getParams) {
  const getParams = computed(() => props.getParams)

  watch(getParams, () => {
    Object.assign(params, {
      ...props.getParams,
      page: 1,
    })
  }, { deep: true })
}

if (props.overwriteParams) {
  const overwriteParams = computed(() => props.overwriteParams)

  watch(overwriteParams, () => {
    Object.assign(params, {
      page: 1,
    })
    if (params.page === 1)
      get_list()
  }, { deep: true })
}

const rowSelection = computed(() => {
  if (batchColumns.value.length > 0 || props.selectionType || props.exportExcel) {
    return {
      selectedRowKeys: selectedRowKeys.value,
      onSelect,
      onSelectAll,
      getCheckboxProps: props?.getCheckboxProps,
      type: (batchColumns.value.length > 0 || props.exportExcel) ? 'checkbox' : props.selectionType,
    }
  }
  else { return null }
}) as ComputedRef<TableProps['rowSelection']>

const hasSelectedRow = computed(() => {
  return batchColumns.value.length > 0 && selectedRowKeys.value.length > 0
})

function clickBatchEdit() {
  emit('clickBatchModify', batchColumns.value, selectedRowKeys.value)
}

function initSortable() {
  useSortable(props, randomId, dataSource, rowsKeyIndexMap, expandKeysList)
}

function changePage(page: number, page_size: number) {
  Object.assign(params, {
    page,
    page_size,
  })
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
    <ATable
      :id="`std-table-${randomId}`"
      :columns="pithyColumns"
      :data-source="dataSource"
      :loading="loading"
      :pagination="false"
      :row-key="rowKey"
      :row-selection="rowSelection"
      :scroll="{ x: scrollX }"
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
        <template v-if="column.dataIndex === 'action'">
          <template v-if="!props.disableView && !params.trash">
            <AButton
              type="link"
              size="small"
              @click="$emit('clickView', record[props.rowKey], record)"
            >
              {{ $gettext('View') }}
            </AButton>
            <ADivider
              v-if="!props.disableModify"
              type="vertical"
            />
          </template>

          <template v-if="!props.disableModify && !params.trash">
            <AButton
              type="link"
              size="small"
              @click="$emit('clickEdit', record[props.rowKey], record)"
            >
              {{ $gettext('Modify') }}
            </AButton>
            <ADivider
              v-if="!props.disableDelete"
              type="vertical"
            />
          </template>

          <slot
            name="actions"
            :record="record"
          />

          <template v-if="!props.disableDelete">
            <APopconfirm
              v-if="!params.trash"
              :cancel-text="$gettext('No')"
              :ok-text="$gettext('OK')"
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
              :ok-text="$gettext('OK')"
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
              v-if="params.trash"
              :cancel-text="$gettext('No')"
              :ok-text="$gettext('OK')"
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
        </template>
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
