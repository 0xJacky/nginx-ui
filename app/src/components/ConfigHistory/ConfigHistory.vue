<script setup lang="ts">
import type { Key } from 'ant-design-vue/es/_util/type'
import type { ConfigBackup } from '@/api/config'
import type { GetListResponse } from '@/api/curd'
import { message } from 'ant-design-vue'
import { defineAsyncComponent } from 'vue'
import config from '@/api/config'
import StdPagination from '@/components/StdDesign/StdDataDisplay/StdPagination.vue'
import { datetime } from '../StdDesign/StdDataDisplay/StdTableTransformer'

// Define props for the component
const props = defineProps<{
  filepath: string
}>()

// Define modal props using defineModel with boolean type
const visible = defineModel<boolean>('visible')
const currentContent = defineModel<string>('currentContent')

// Import DiffViewer asynchronously with loading options
const DiffViewer = defineAsyncComponent({
  loader: () => import('./DiffViewer.vue'),
  loadingComponent: {
    template: '<div class="async-loading"><ASpin /></div>',
  },
  delay: 200,
  timeout: 10000,
  errorComponent: {
    template: '<div class="async-error"><AAlert type="error" message="Failed to load component" /></div>',
  },
})

const loading = ref(false)
const records = ref<ConfigBackup[]>([])
const showDiffViewer = ref(false)
const pagination = ref({
  total: 0,
  per_page: 10,
  current_page: 1,
  total_pages: 0,
})
const selectedRowKeys = ref<Key[]>([])
const selectedRecords = ref<ConfigBackup[]>([])

// Watch for changes in modal visibility and filepath to fetch data
watch(() => [visible.value, props.filepath], ([newVisible, newPath]) => {
  if (newVisible && newPath) {
    fetchHistoryList()
  }
}, { immediate: true })

// Table column definitions
const columns = [
  {
    title: () => $gettext('Modified At'),
    dataIndex: 'created_at',
    customRender: datetime,
  },
]

// Fetch history records list
async function fetchHistoryList() {
  if (!props.filepath)
    return

  loading.value = true
  try {
    const response = await config.get_history(props.filepath)
    const data = response as GetListResponse<ConfigBackup>
    records.value = data.data || []

    if (data.pagination) {
      pagination.value = data.pagination
    }
  }
  catch (error) {
    message.error($gettext('Failed to load history records'))
    console.error('Failed to fetch config backup list:', error)
  }
  finally {
    loading.value = false
  }
}

// Handle pagination changes
function changePage(page: number, pageSize: number) {
  pagination.value.current_page = page
  pagination.value.per_page = pageSize
  fetchHistoryList()
}

// Row selection handler
const rowSelection = computed(() => ({
  selectedRowKeys: selectedRowKeys.value,
  hideSelectAll: true,
  onChange: (keys: Key[], selectedRows: ConfigBackup[]) => {
    // Limit to maximum of two records
    if (keys.length > 2) {
      return
    }
    selectedRowKeys.value = keys
    selectedRecords.value = selectedRows
  },
  getCheckboxProps: (record: ConfigBackup) => ({
    disabled: selectedRowKeys.value.length >= 2 && !selectedRowKeys.value.includes(record.id as Key),
  }),
}))

// Compare selected records
function compareSelected() {
  if (selectedRecords.value.length > 0) {
    showDiffViewer.value = true
  }
}

// Close modal and reset selection
function handleClose() {
  showDiffViewer.value = false
  selectedRowKeys.value = []
  selectedRecords.value = []
  visible.value = false
}

// Dynamic button text based on selection count
const compareButtonText = computed(() => {
  if (selectedRowKeys.value.length === 0)
    return $gettext('Compare')
  if (selectedRowKeys.value.length === 1)
    return $gettext('Compare with Current')
  return $gettext('Compare Selected')
})
</script>

<template>
  <div>
    <AModal
      v-model:open="visible"
      :title="$gettext('Configuration History')"
      :footer="null"
      @cancel="handleClose"
    >
      <div class="history-container">
        <ATable
          :loading="loading"
          :columns="columns"
          :data-source="records"
          :row-selection="rowSelection"
          row-key="id"
          size="small"
          :pagination="false"
        />

        <div class="history-footer">
          <StdPagination
            :pagination="pagination"
            :loading="loading"
            @change="changePage"
          />

          <div class="actions">
            <AButton
              type="primary"
              :disabled="selectedRowKeys.length === 0"
              @click="compareSelected"
            >
              {{ compareButtonText }}
            </AButton>
            <AButton @click="handleClose">
              {{ $gettext('Close') }}
            </AButton>
          </div>
        </div>
      </div>
    </AModal>
    <DiffViewer
      v-model:visible="showDiffViewer"
      v-model:current-content="currentContent"
      :records="selectedRecords"
      @restore="visible = false"
    />
  </div>
</template>

<style lang="less" scoped>
.history-container {
  display: flex;
  flex-direction: column;
  height: 100%;
}

.history-footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-top: 16px;
}

.actions {
  display: flex;
  gap: 8px;
}

.async-loading,
.async-error {
  display: flex;
  justify-content: center;
  align-items: center;
  padding: 20px;
  min-height: 200px;
}
</style>
