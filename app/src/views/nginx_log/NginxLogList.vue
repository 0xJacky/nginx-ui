<script setup lang="tsx">
import type { CustomRenderArgs, StdTableColumn } from '@uozi-admin/curd'
import type { NginxLogData } from '@/api/nginx_log'
import { CheckCircleOutlined, SyncOutlined } from '@ant-design/icons-vue'
import { StdCurd } from '@uozi-admin/curd'
import { Badge, Tag, Tooltip } from 'ant-design-vue'
import dayjs from 'dayjs'
import nginxLog from '@/api/nginx_log'
import { useWebSocketEventBus } from '@/composables/useWebSocketEventBus'
import { useGlobalStore } from '@/pinia'
import { useIndexProgress } from './composables/useIndexProgress'
import IndexProgressBar from './indexing/components/IndexProgressBar.vue'
import IndexManagement from './indexing/IndexManagement.vue'

const router = useRouter()
const stdCurdRef = ref()
const indexManagementRef = ref()

// WebSocket event bus and global store
const { subscribe } = useWebSocketEventBus()
const globalStore = useGlobalStore()
const { nginxLogStatus, processingStatus } = storeToRefs(globalStore)

// Index progress tracking
const { getProgressForFile, isGlobalIndexing, globalProgress } = useIndexProgress()

// Subscribe to events
onMounted(() => {
  // Subscribe to processing status events
  subscribe('processing_status', data => {
    processingStatus.value = data
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

const columns: StdTableColumn[] = [
  {
    title: () => $gettext('Type'),
    dataIndex: 'type',
    customRender: (args: CustomRenderArgs) => {
      return args.record?.type === 'access' ? <Tag color="green">{ $gettext('Access Log') }</Tag> : <Tag color="orange">{ $gettext('Error Log') }</Tag>
    },
    sorter: true,
    search: {
      type: 'select',
      select: {
        options: [
          {
            label: () => $gettext('Access Log'),
            value: 'access',
          },
          {
            label: () => $gettext('Error Log'),
            value: 'error',
          },
        ],
      },
    },
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
  {
    title: () => $gettext('Index Status'),
    dataIndex: 'index_status',
    customRender: (args: CustomRenderArgs) => {
      const record = args.record
      if (!record)
        return null

      // Check if file is currently being indexed with progress
      const progress = getProgressForFile(record.path)
      if (progress) {
        return (
          <div style="min-width: 200px; padding: 6px 0 8px 0;">
            <IndexProgressBar progress={progress} size="small" />
          </div>
        )
      }

      // Show regular status badges when not actively indexing
      switch (record.index_status) {
        case 'indexed':
          return (
            <Tooltip title={$gettext('Indexed and searchable')}>
              <Badge status="success" text={$gettext('Indexed')} />
            </Tooltip>
          )
        case 'indexing':
          return (
            <Tooltip title={$gettext('Currently being indexed')}>
              <Badge status="processing" text={$gettext('Indexing')} />
            </Tooltip>
          )
        case 'not_indexed':
        default:
          return (
            <Tooltip title={$gettext('Not indexed for search')}>
              <Badge status="default" text={$gettext('Not Indexed')} />
            </Tooltip>
          )
      }
    },
    sorter: true,
    search: {
      type: 'select',
      select: {
        options: [
          {
            label: () => $gettext('Indexed'),
            value: 'true',
          },
          {
            label: () => $gettext('Indexing'),
            value: 'indexing',
          },
          {
            label: () => $gettext('Not Indexed'),
            value: 'false',
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

      const tooltipContent = (
        <div>
          <div>{lastIndexed.format('YYYY-MM-DD HH:mm:ss')}</div>
          {durationText && (
            <div class="text-xs text-gray-100 dark:text-gray-300 mt-1">
              Duration:
              {' '}
              {durationText.slice(1, -1)}
            </div>
          )}
        </div>
      )

      return (
        <Tooltip title={tooltipContent}>
          <span>
            {displayText}
            {durationText && <span class="text-xs text-gray-500 dark:text-gray-400 ml-1">{durationText}</span>}
            {statusIcon}
          </span>
        </Tooltip>
      )
    },
    sorter: true,
    width: 220,
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
      const duration = end.diff(start, 'day')

      // Format duration display
      let durationText = ''
      if (duration === 0) {
        durationText = $gettext('Today')
      }
      else if (duration === 1) {
        durationText = '1 day'
      }
      else if (duration < 30) {
        durationText = `${duration} days`
      }
      else if (duration < 365) {
        const months = Math.floor(duration / 30)
        durationText = `${months} month${months > 1 ? 's' : ''}`
      }
      else {
        const years = Math.floor(duration / 365)
        durationText = `${years} year${years > 1 ? 's' : ''}`
      }

      return (
        <Tooltip title={durationText}>
          <span>
            {start.format('YYYY-MM-DD HH:mm:ss')}
            {' '}
            ~
            {' '}
            {end.format('YYYY-MM-DD HH:mm:ss')}
          </span>
        </Tooltip>
      )
    },
    width: 380,
  },
  {
    title: () => $gettext('Actions'),
    dataIndex: 'actions',
    fixed: 'right',
    width: 250,
  },
]

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
</script>

<template>
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
  >
    <template #beforeListActions>
      <div class="flex items-center gap-4">
        <!-- Global indexing progress -->
        <div v-if="isGlobalIndexing" class="flex items-center space-x-4">
          <div class="flex items-center text-blue-500">
            <SyncOutlined spin class="mr-2" />
            <span>{{ $gettext('Indexing logs...') }}</span>
          </div>
          <div v-if="globalProgress.totalFiles > 0" class="text-sm text-gray-600 dark:text-gray-400">
            <span>
              {{ globalProgress.completedFiles }} / {{ globalProgress.totalFiles }}
              {{ $gettext('files') }}
            </span>
          </div>
        </div>

        <!-- Index Management -->
        <IndexManagement
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

      <!-- Rebuild File Index Action -->
      <AButton
        type="link"
        size="small"
        :disabled="processingStatus.nginx_log_indexing"
        @click="rebuildFileIndex(record)"
      >
        {{ $gettext('Rebuild') }}
      </AButton>
    </template>
  </StdCurd>
</template>

<style scoped lang="less">

</style>
