<script setup lang="tsx">
import type { CustomRenderArgs, StdTableColumn } from '@uozi-admin/curd'
import type { NginxLogData } from '@/api/nginx_log'
import type { TabOption } from '@/components/TabFilter'
import { CheckCircleOutlined, ExclamationCircleOutlined, SyncOutlined } from '@ant-design/icons-vue'
import { StdCurd } from '@uozi-admin/curd'
import { useRouteQuery } from '@vueuse/router'
import { Badge, Tag, Tooltip } from 'ant-design-vue'
import dayjs from 'dayjs'
import nginxLog from '@/api/nginx_log'
import { TabFilter } from '@/components/TabFilter'
import { useWebSocketEventBus } from '@/composables/useWebSocketEventBus'
import { useGlobalStore } from '@/pinia'
import IndexingSettingsModal from './components/IndexingSettingsModal.vue'
import { useIndexProgress } from './composables/useIndexProgress'
import IndexProgressBar from './indexing/components/IndexProgressBar.vue'
import IndexManagement from './indexing/IndexManagement.vue'

const { message } = App.useApp()

const router = useRouter()
const stdCurdRef = ref()
const indexManagementRef = ref()
const indexingSettingsModalVisible = ref(false)
const advancedIndexingEnabled = ref(false)
const enableIndexingLoading = ref(false)

// WebSocket event bus and global store
const { subscribe } = useWebSocketEventBus()
const globalStore = useGlobalStore()
const { nginxLogStatus, processingStatus } = storeToRefs(globalStore)

// Index progress tracking
const { getProgressForFile, isGlobalIndexing } = useIndexProgress()

// Tab filter for log types
const activeLogType = useRouteQuery('type', 'access')

const tabOptions: TabOption[] = [
  {
    key: 'access',
    label: $gettext('Access Logs'),
    icon: h(CheckCircleOutlined),
    color: '#52c41a',
  },
  {
    key: 'error',
    label: $gettext('Error Logs'),
    icon: h(ExclamationCircleOutlined),
    color: '#ff4d4f',
  },
]

// Auto-refresh timer for indexing status updates
let autoRefreshTimer: ReturnType<typeof setInterval> | null = null

// Start auto refresh when indexing begins
function startAutoRefresh() {
  if (autoRefreshTimer)
    return // Already running

  autoRefreshTimer = setInterval(() => {
    if (stdCurdRef.value && processingStatus.value?.nginx_log_indexing) {
      stdCurdRef.value.refresh()
    }
  }, 2000) // Refresh every 2 seconds during indexing
}

// Stop auto refresh when indexing completes
function stopAutoRefresh() {
  if (autoRefreshTimer) {
    clearInterval(autoRefreshTimer)
    autoRefreshTimer = null
  }
}

// Check advanced indexing status on mount
async function checkAdvancedIndexingStatus() {
  try {
    const response = await nginxLog.getAdvancedIndexingStatus()
    advancedIndexingEnabled.value = response.enabled
  }
  catch (error) {
    console.error('Failed to check advanced indexing status:', error)
  }
}

// Subscribe to events
onMounted(async () => {
  // Check advanced indexing status
  await checkAdvancedIndexingStatus()

  // Subscribe to processing status events
  subscribe('processing_status', data => {
    const wasIndexing = processingStatus.value?.nginx_log_indexing
    processingStatus.value = data

    // Start/stop auto refresh based on indexing status
    if (data?.nginx_log_indexing && !wasIndexing) {
      // Indexing started
      startAutoRefresh()
    }
    else if (!data?.nginx_log_indexing && wasIndexing) {
      // Indexing stopped
      stopAutoRefresh()
      // Final refresh to update status
      setTimeout(() => {
        if (stdCurdRef.value) {
          stdCurdRef.value.refresh()
        }
      }, 500)
    }
  })

  // Subscribe to nginx log status events (backward compatibility)
  subscribe('nginx_log_status', data => {
    nginxLogStatus.value = data
  })

  // Subscribe to index ready events to refresh the list
  subscribe('nginx_log_index_ready', () => {
    // Refresh the table data
    if (stdCurdRef.value) {
      setTimeout(() => {
        stdCurdRef.value.refresh()
      }, 1000)
    }
  })
})

onUnmounted(() => {
  // Clean up auto refresh timer
  stopAutoRefresh()
})

// Base columns that are always visible
const baseColumns: StdTableColumn[] = [
  {
    title: () => $gettext('Type'),
    dataIndex: 'type',
    customRender: (args: CustomRenderArgs) => {
      return args.record?.type === 'access' ? <Tag color="green">{ $gettext('Access Log') }</Tag> : <Tag color="orange">{ $gettext('Error Log') }</Tag>
    },
    sorter: true,
    width: 120,
  },
  {
    title: () => $gettext('Name'),
    dataIndex: 'name',
    sorter: true,
    search: {
      type: 'input',
    },
    width: 200,
  },
  {
    title: () => $gettext('Path'),
    dataIndex: 'path',
    sorter: true,
    search: {
      type: 'input',
    },
    ellipsis: true,
  },
]

// Index-related columns only for Access logs
const indexColumns: StdTableColumn[] = [
  {
    title: () => $gettext('Index Status'),
    dataIndex: 'index_status',
    customRender: (args: CustomRenderArgs) => {
      const record = args.record
      if (!record)
        return null

      // Check if file is currently being indexed with progress
      const progress = getProgressForFile(record.path)
      // Show progress bar for actual indexing progress (not status updates)
      if (progress && progress.stage !== 'status_update') {
        return (
          <div style="min-width: 200px; padding: 6px 0 8px 0;">
            <IndexProgressBar progress={progress} size="small" />
          </div>
        )
      }

      // Show regular status badges when not actively indexing
      switch (record.index_status) {
        case 'indexed':
        case 'ready': // Treat 'ready' as 'indexed' for backward compatibility
          return (
            <Badge status="success" text={$gettext('Indexed')} />
          )
        case 'indexing':
          return (
            <Badge status="processing" text={$gettext('Indexing')} />
          )
        case 'error':
          return (
            <Tooltip title={record.error_message || $gettext('Index failed')}>
              <Badge status="error" text={$gettext('Error')} />
            </Tooltip>
          )
        case 'queued': {
          const queueText = record.queue_position
            ? `${$gettext('Queued')} (#${record.queue_position})`
            : $gettext('Queued')
          return (
            <Badge status="processing" text={queueText} />
          )
        }
        case 'not_indexed':
        default:
          return (
            <Badge status="default" text={$gettext('Not Indexed')} />
          )
      }
    },
    sorter: true,
    search: {
      type: 'select',
      select: {
        options: [
          {
            label: () => $gettext('Not Indexed'),
            value: 'not_indexed',
          },
          {
            label: () => $gettext('Queued'),
            value: 'queued',
          },
          {
            label: () => $gettext('Indexing'),
            value: 'indexing',
          },
          {
            label: () => $gettext('Indexed'),
            value: 'indexed',
          },
          {
            label: () => $gettext('Error'),
            value: 'error',
          },
        ],
      },
    },
    width: 250,
  },
  {
    title: () => $gettext('Last Indexed'),
    dataIndex: 'last_indexed',
    customRender: (args: CustomRenderArgs) => {
      const record = args.record
      if (!record || !record.last_indexed)
        return <span class="text-gray-400 dark:text-gray-500">-</span>

      const lastIndexed = dayjs.unix(record.last_indexed)
      const displayText = lastIndexed.format('YYYY-MM-DD HH:mm')
      const statusIcon = <CheckCircleOutlined class="text-green-500 ml-1" />

      // Format duration if available
      let durationText = ''
      if (record.index_duration) {
        const duration = record.index_duration
        if (duration < 1000) {
          durationText = `(${duration}ms)`
        }
        else if (duration < 60000) {
          durationText = `(${(duration / 1000).toFixed(1)}s)`
        }
        else {
          const minutes = Math.floor(duration / 60000)
          const seconds = Math.floor((duration % 60000) / 1000)
          durationText = `(${minutes}m ${seconds}s)`
        }
      }

      return (
        <span>
          {displayText}
          {durationText && <span class="text-xs text-gray-500 dark:text-gray-400 ml-1">{durationText}</span>}
          {statusIcon}
        </span>
      )
    },
    sorter: true,
    width: 250,
  },
  {
    title: () => $gettext('Document Count'),
    dataIndex: 'document_count',
    customRender: (args: CustomRenderArgs) => {
      const record = args.record
      if (!record || !record.document_count) {
        return <span class="text-gray-400 dark:text-gray-500">-</span>
      }
      return <span>{record.document_count.toLocaleString()}</span>
    },
    sorter: true,
    width: 130,
  },
  {
    title: () => $gettext('Time Range'),
    dataIndex: 'timerange',
    customRender: (args: CustomRenderArgs) => {
      const record = args.record
      if (!record || !record.has_timerange || !record.timerange_start || !record.timerange_end) {
        return <span class="text-gray-400 dark:text-gray-500">-</span>
      }

      const start = dayjs.unix(record.timerange_start)
      const end = dayjs.unix(record.timerange_end)

      return (
        <span>
          {start.format('YYYY-MM-DD HH:mm:ss')}
          {' '}
          ~
          {' '}
          {end.format('YYYY-MM-DD HH:mm:ss')}
        </span>
      )
    },
    width: 380,
  },
]

// Actions column
const actionsColumn: StdTableColumn = {
  title: () => $gettext('Actions'),
  dataIndex: 'actions',
  fixed: 'right',
  width: 250,
}

// Computed columns based on active log type
const columns = computed(() => {
  const cols = [...baseColumns]

  // Only show index-related columns for Access logs
  if (activeLogType.value === 'access') {
    cols.push(...indexColumns)
  }

  cols.push(actionsColumn)
  return cols
})

function viewLog(record: NginxLogData) {
  router.push({
    path: `/nginx_log/${record.type}`,
    query: {
      path: record.path,
    },
  })
}

function rebuildFileIndex(record: NginxLogData) {
  if (indexManagementRef.value && record.path) {
    indexManagementRef.value.rebuildFileIndex(record.path)
  }
}

async function refreshTable() {
  stdCurdRef.value.refresh()
}

function showIndexingSettingsModal() {
  indexingSettingsModalVisible.value = true
}

async function enableAdvancedIndexing() {
  enableIndexingLoading.value = true
  try {
    await nginxLog.enableAdvancedIndexing()
    advancedIndexingEnabled.value = true
    indexingSettingsModalVisible.value = false

    // Show success message
    message.success($gettext('Advanced indexing enabled successfully'))

    // Start rebuild all indexes
    try {
      await nginxLog.rebuildIndex()
      message.success($gettext('Index rebuild initiated'))
    }
    catch (rebuildError) {
      console.error('Failed to rebuild index:', rebuildError)
      message.warning($gettext('Advanced indexing enabled but failed to start rebuild'))
    }

    // Refresh table to show updated indexing status
    setTimeout(() => {
      refreshTable()
    }, 500)
  }
  catch (error) {
    console.error('Failed to enable advanced indexing:', error)
    message.error($gettext('Failed to enable advanced indexing'))
  }
  finally {
    enableIndexingLoading.value = false
  }
}

function cancelIndexingSettings() {
  indexingSettingsModalVisible.value = false
}
</script>

<template>
  <div>
    <StdCurd
      ref="stdCurdRef"
      :title="$gettext('Log List')"
      :columns="columns"
      :api="nginxLog"
      disable-add
      disable-export
      disable-delete
      disable-trash
      disable-view
      disable-edit
      :overwrite-params="{
        type: activeLogType,
      }"
    >
      <template #beforeSearch>
        <TabFilter
          v-model:active-key="activeLogType"
          :options="tabOptions"
          size="middle"
        />
      </template>

      <template #beforeListActions>
        <div class="flex items-center gap-4">
          <!-- Global indexing progress -->
          <div v-if="isGlobalIndexing" class="flex items-center">
            <div class="flex items-center text-blue-500">
              <SyncOutlined spin class="mr-2" />
              <span>{{ $gettext('Indexing logs...') }}</span>
            </div>
          </div>

          <!-- Advanced Indexing Toggle - only for Access logs -->
          <div v-if="activeLogType === 'access' && !advancedIndexingEnabled" class="flex items-center">
            <AButton
              type="link"
              size="small"
              @click="showIndexingSettingsModal"
            >
              {{ $gettext('Enable Advanced Indexing') }}
            </AButton>
          </div>

          <!-- Index Management - only for Access logs when advanced indexing is enabled -->
          <IndexManagement
            v-if="activeLogType === 'access' && advancedIndexingEnabled"
            ref="indexManagementRef"
            :disabled="processingStatus.nginx_log_indexing"
            :indexing="isGlobalIndexing || processingStatus.nginx_log_indexing"
            @refresh="refreshTable"
          />
        </div>
      </template>
      <template #beforeActions="{ record }">
        <AButton type="link" size="small" @click="viewLog(record)">
          {{ $gettext('View') }}
        </AButton>

        <!-- Rebuild File Index Action - only for Access logs with advanced indexing enabled -->
        <AButton
          v-if="record.type === 'access' && advancedIndexingEnabled"
          type="link"
          size="small"
          :disabled="processingStatus.nginx_log_indexing"
          @click="rebuildFileIndex(record)"
        >
          {{ $gettext('Rebuild') }}
        </AButton>
      </template>
    </StdCurd>

    <!-- Advanced Indexing Settings Modal -->
    <IndexingSettingsModal
      v-model:visible="indexingSettingsModalVisible"
      :loading="enableIndexingLoading"
      @confirm="enableAdvancedIndexing"
      @cancel="cancelIndexingSettings"
    />
  </div>
</template>

<style scoped lang="less">

</style>
