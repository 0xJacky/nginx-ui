<script setup lang="ts">
import { message } from 'ant-design-vue'
import { HolderOutlined } from '@ant-design/icons-vue'
import { useGettext } from 'vue3-gettext'
import type { ComputedRef } from 'vue'
import type { SorterResult } from 'ant-design-vue/lib/table/interface'
import StdPagination from './StdPagination.vue'
import StdDataEntry from '@/components/StdDesign/StdDataEntry'
import type { Pagination } from '@/api/curd'
import type { Column } from '@/components/StdDesign/types'
import exportCsvHandler from '@/components/StdDesign/StdDataDisplay/methods/exportCsv'
import useSortable from '@/components/StdDesign/StdDataDisplay/methods/sortable'
import type Curd from '@/api/curd'

export interface StdTableProps {
  title?: string
  mode?: string
  rowKey?: string
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  api: Curd<any>
  columns: Column[]
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  getParams?: Record<string, any>
  size?: string
  disableQueryParams?: boolean
  disableSearch?: boolean
  pithy?: boolean
  exportCsv?: boolean
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  overwriteParams?: Record<string, any>
  disabledModify?: boolean
  selectionType?: string
  sortable?: boolean
  disableDelete?: boolean
  disablePagination?: boolean
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  selectedRowKeys?: any | any[]
  sortableMoveHook?: (oldRow: number[], newRow: number[]) => boolean
  scrollX?: string | number
}

const props = withDefaults(defineProps<StdTableProps>(), {
  rowKey: 'id',
})

const emit = defineEmits(['onSelected', 'onSelectedRecord', 'clickEdit', 'update:selectedRowKeys', 'clickBatchModify'])
const { $gettext } = useGettext()
const route = useRoute()
const dataSource = ref([])
const expandKeysList = ref([])
const rowsKeyIndexMap = ref({})
const loading = ref(true)

// This can be useful if there are more than one StdTable in the same page.
const randomId = ref(Math.random().toString(36).substring(2, 8))

const pagination: Pagination = reactive({
  total: 1,
  per_page: 10,
  current_page: 1,
  total_pages: 1,
})

const params = reactive({
  ...props.getParams,
})

const selectedKeysLocalBuffer = ref([])

const selectedRowKeysBuffer = computed({
  get() {
    return props.selectedRowKeys || selectedKeysLocalBuffer.value
  },
  set(v) {
    selectedKeysLocalBuffer.value = v
    emit('update:selectedRowKeys', v)
  },
})

const searchColumns = computed(() => {
  const _searchColumns = []

  props.columns?.forEach(column => {
    if (column.search)
      _searchColumns.push(column)
  })

  return _searchColumns
})

const pithyColumns = computed(() => {
  if (props.pithy) {
    return props.columns?.filter(c => {
      return c.pithy === true && !c.hidden
    })
  }

  return props.columns?.filter(c => {
    return !c.hidden
  })
}) as ComputedRef<Column[]>

const batchColumns = computed(() => {
  const batch = []

  props.columns?.forEach(column => {
    if (column.batch)
      batch.push(column)
  })

  return batch
})

onMounted(() => {
  if (!props.disableQueryParams)
    Object.assign(params, route.query)

  get_list()

  if (props.sortable)
    initSortable()
})

defineExpose({
  get_list,
})

function destroy(id) {
  props.api!.destroy(id).then(() => {
    get_list()
    message.success($gettext('Deleted successfully'))
  }).catch(e => {
    message.error($gettext(e?.message ?? 'Server error'))
  })
}

function get_list(page_num = null, page_size = 20) {
  loading.value = true
  if (page_num) {
    params.page = page_num
    params.page_size = page_size
  }
  props.api?.get_list(params).then(async r => {
    dataSource.value = r.data
    rowsKeyIndexMap.value = {}
    if (props.sortable)

      buildIndexMap(r.data)

    if (r.pagination)
      Object.assign(pagination, r.pagination)

    loading.value = false
  }).catch(e => {
    message.error(e?.message ?? $gettext('Server error'))
  })
}
function buildIndexMap(data, level: number = 0, index: number = 0, total: number[] = []) {
  if (data && data.length > 0) {
    data.forEach(v => {
      v.level = level

      const current_indexes = [...total, index++]

      rowsKeyIndexMap.value[v.id] = current_indexes
      if (v.children)
        buildIndexMap(v.children, level + 1, 0, current_indexes)
    })
  }
}
function orderPaginationChange(_pagination: Pagination, filters, sorter: SorterResult) {
  if (sorter) {
    selectedRowKeysBuffer.value = []
    params.order_by = sorter.field
    params.sort = sorter.order === 'ascend' ? 'asc' : 'desc'
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
  if (_pagination)
    selectedRowKeysBuffer.value = []
}

function expandedTable(keys) {
  expandKeysList.value = keys
}

const crossPageSelect = {}

async function onSelectChange(_selectedRowKeys) {
  const page = params.page || 1

  crossPageSelect[page] = await _selectedRowKeys

  let t = []
  Object.keys(crossPageSelect).forEach(v => {
    t.push(...crossPageSelect[v])
  })

  const n = [..._selectedRowKeys]

  t = t.concat(n)

  // console.log(crossPageSelect)
  const set = new Set(t)

  selectedRowKeysBuffer.value = Array.from(set)
  emit('onSelected', selectedRowKeysBuffer.value)
}

function onSelect(record) {
  emit('onSelectedRecord', record)
}

const router = useRouter()

const reset_search = async () => {
  Object.keys(params).forEach(v => {
    delete params[v]
  })

  Object.assign(params, {
    ...props.getParams,
  })

  router.push({ query: {} }).catch(() => {
  })
}

watch(params, () => {
  if (!props.disableQueryParams)
    router.push({ query: params })

  get_list()
})

const rowSelection = computed(() => {
  if (batchColumns.value.length > 0 || props.selectionType) {
    return {
      selectedRowKeys: selectedRowKeysBuffer.value,
      onChange: onSelectChange,
      onSelect,
      type: batchColumns.value.length > 0 ? 'checkbox' : props.selectionType,
    }
  }
  else {
    return null
  }
})

const hasSelectedRow = computed(() => {
  return batchColumns.value.length > 0 && selectedRowKeysBuffer.value.length > 0
})

function clickBatchEdit() {
  emit('clickBatchModify', batchColumns.value, selectedRowKeysBuffer.value)
}

function initSortable() {
  useSortable(props, randomId, dataSource, rowsKeyIndexMap, expandKeysList)
}

function export_csv() {
  exportCsvHandler(props, pithyColumns)
}
</script>

<template>
  <div class="std-table">
    <StdDataEntry
      v-if="!disableSearch && searchColumns.length"
      :data-list="searchColumns"
      :data-source="params"
      layout="inline"
    >
      <template #action>
        <ASpace class="action-btn">
          <AButton
            v-if="props.exportCsv"
            type="primary"
            ghost
            @click="export_csv"
          >
            {{ $gettext('Export') }}
          </AButton>
          <AButton @click="reset_search">
            {{ $gettext('Reset') }}
          </AButton>
          <AButton
            v-if="hasSelectedRow"
            @click="clickBatchEdit"
          >
            {{ $gettext('Batch Modify') }}
          </AButton>
        </ASpace>
      </template>
    </StdDataEntry>
    <ATable
      id="std-table"
      :columns="pithyColumns"
      :data-source="dataSource"
      :loading="loading"
      :pagination="false"
      :row-key="rowKey"
      :row-selection="rowSelection"
      :scroll="{ x: scrollX }"
      :size="size"
      :expanded-row-keys="expandKeysList"
      @change="orderPaginationChange"
      @expanded-rows-change="expandedTable"
    >
      <template #bodyCell="{ text, record, column }">
        <template v-if="column.handle === true">
          <span class="ant-table-drag-icon"><HolderOutlined /></span>
          {{ text }}
        </template>
        <template v-if="column.dataIndex === 'action'">
          <AButton
            v-if="!props.disabledModify"
            type="link"
            size="small"
            @click="$emit('clickEdit', record[props.rowKey], record)"
          >
            {{ $gettext('Modify') }}
          </AButton>
          <slot
            name="actions"
            :record="record"
          />
          <template v-if="!props.disableDelete">
            <ADivider type="vertical" />
            <APopconfirm
              :cancel-text="$gettext('No')"
              :ok-text="$gettext('OK')"
              :title="$gettext('Are you sure you want to delete?')"
              @confirm="destroy(record[rowKey])"
            >
              <AButton
                type="link"
                size="small"
              >
                {{ $gettext('Delete') }}
              </AButton>
            </APopconfirm>
          </template>
        </template>
      </template>
    </ATable>
    <StdPagination
      :size="size"
      :pagination="pagination"
      @change="get_list"
      @change-page-size="orderPaginationChange"
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

.std-table {
  .ant-table-wrapper {
    // overflow-x: scroll;
  }
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
