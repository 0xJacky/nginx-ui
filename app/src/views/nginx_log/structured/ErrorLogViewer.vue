<script setup lang="ts">
import type { TableSorterResult as SorterResult, TablePaginationConfig } from 'antdv-next'
import type { ErrorLogEntry, PreflightResponse } from '@/api/nginx_log'
import { DownOutlined, QuestionCircleOutlined, ReloadOutlined } from '@antdv-next/icons'
import { DatePicker } from 'antdv-next'
import dayjs from 'dayjs'
import nginx_log from '@/api/nginx_log'
import { useWebSocketEventBusStore } from '@/pinia'
import LoadingState from '../components/LoadingState.vue'
import { useIndexProgress } from '../composables/useIndexProgress'
import { errorLevelColor, errorLevelOptions, errorLevelParam, errorLogContext, errorLogRequest } from './errorLog'
import { isPathValidationError, timePresetRange, timePresets } from './searchHelpers'
import { getInitialStructuredTimeRange } from './timeRange'

const props = defineProps<{
  logPath?: string
}>()

const { RangePicker } = DatePicker

interface ErrorLogRow extends ErrorLogEntry {
  key: number
}

const { message } = App.useApp()

const websocketEventBus = useWebSocketEventBusStore()
let indexReadySubscriptionId: string | null = null

const { isFileIndexing } = useIndexProgress()

const logPath = computed(() => props.logPath || undefined)

const timeRange = ref({
  start: null as dayjs.Dayjs | null,
  end: null as dayjs.Dayjs | null,
})
const preflightResponse = ref<PreflightResponse | null>(null)
const query = ref('')
const levels = ref<string[]>([])
const entries = ref<ErrorLogRow[]>([])
const total = ref(0)
const took = ref(0)
const hasSearched = ref(false)
const searchLoading = ref(false)
const indexingStatus = ref<'idle' | 'indexing' | 'indexed' | 'failed'>('idle')
const currentPage = ref(1)
const pageSize = ref(50)
const sortOrder = ref<'asc' | 'desc'>('desc')

// The search box syntax, see logstore.ParseSearchText
const syntaxHelp = computed(() => [
  $gettext('Words match anywhere in the line; all of them must match. Quotes keep a phrase together, a leading - excludes.'),
  'level:error  level:crit  -level:warn',
  'ip:10.0.0.0/8  path:/api  method:POST',
  'referer:google  country:CN',
])

const dateRange = computed({
  get: () => {
    const { start, end } = timeRange.value
    return (start && end)
      ? [start, end] as [dayjs.Dayjs, dayjs.Dayjs]
      : undefined
  },
  set: value => {
    if (value && Array.isArray(value) && value.length === 2) {
      timeRange.value.start = value[0]
      timeRange.value.end = value[1]
    }
  },
})

const isCurrentFileIndexing = computed(() => logPath.value
  ? isFileIndexing(logPath.value)
  : false)
const isFileAvailable = computed(() => preflightResponse.value?.available === true)
const isReady = computed(() => indexingStatus.value === 'indexed')
const isFailed = computed(() => indexingStatus.value === 'failed')
const shouldShowControls = computed(() => isFileAvailable.value && !isCurrentFileIndexing.value)
const shouldShowIndexingSpinner = computed(() =>
  isCurrentFileIndexing.value || (indexingStatus.value === 'indexing' && !isFileAvailable.value))
const shouldShowResults = computed(() =>
  isFileAvailable.value && !isCurrentFileIndexing.value && hasSearched.value)

const columns = computed(() => [
  {
    title: $gettext('Time'),
    dataIndex: 'timestamp',
    key: 'timestamp',
    width: 170,
    fixed: 'left' as const,
    sorter: true,
    sortOrder: sortOrder.value === 'asc'
      ? 'ascend' as const
      : 'descend' as const,
  },
  {
    title: $gettext('Level'),
    dataIndex: 'level',
    key: 'level',
    width: 90,
  },
  {
    title: $gettext('Client'),
    dataIndex: 'ip',
    key: 'ip',
    width: 260,
  },
  {
    title: $gettext('Request'),
    dataIndex: 'path',
    key: 'request',
    width: 300,
    ellipsis: true,
  },
  {
    title: $gettext('Message'),
    dataIndex: 'message',
    key: 'message',
  },
])

async function performSearch() {
  if (!timeRange.value.start || !timeRange.value.end)
    return

  searchLoading.value = true
  try {
    const offset = (currentPage.value - 1) * pageSize.value
    const result = await nginx_log.searchErrorLog({
      start_time: timeRange.value.start.unix(),
      end_time: timeRange.value.end.unix(),
      query: query.value.trim() || undefined,
      level: errorLevelParam(levels.value),
      limit: pageSize.value,
      offset,
      sort_by: 'timestamp',
      sort_order: sortOrder.value,
      log_path: logPath.value,
    })

    entries.value = (result.entries ?? []).map((entry, index) => ({ ...entry, key: offset + index }))
    total.value = result.total || 0
    took.value = result.took || 0
    hasSearched.value = true

    if (result.query_warnings?.length) {
      message.warning($gettext('Searched as plain text: %{tokens}', {
        tokens: result.query_warnings.map(w => w.token).join(', '),
      }))
    }
  }
  catch (error: unknown) {
    entries.value = []
    total.value = 0
    if (!isPathValidationError(error))
      console.error('Failed to search the error log:', error)
  }
  finally {
    searchLoading.value = false
  }
}

function search() {
  currentPage.value = 1
  performSearch()
}

function resetSearch() {
  query.value = ''
  // Clearing the levels searches through their watcher
  if (levels.value.length > 0)
    levels.value = []
  else
    search()
}

const timePresetItems = computed(() => timePresets.map(preset => ({
  key: preset.key,
  label: preset.label(),
})))

function applyTimePresetByKey(key: string) {
  const range = timePresetRange(key)
  if (range)
    timeRange.value = range
}

function handleTableChange(
  pagination: TablePaginationConfig,
  _filters: Record<string, unknown>,
  sorter: SorterResult<ErrorLogRow> | SorterResult<ErrorLogRow>[],
) {
  const singleSorter = Array.isArray(sorter)
    ? sorter[0]
    : sorter
  // Clearing the sort goes back to the newest entries first
  const nextOrder = singleSorter?.order === 'ascend'
    ? 'asc'
    : 'desc'

  if (nextOrder !== sortOrder.value || (pagination.pageSize && pagination.pageSize !== pageSize.value)) {
    sortOrder.value = nextOrder
    pageSize.value = pagination.pageSize ?? pageSize.value
    currentPage.value = 1
  }
  else if (pagination.current) {
    currentPage.value = pagination.current
  }

  performSearch()
}

// Ask whether the log is indexed and anchor the time range to its entries
async function loadPreflight(): Promise<boolean> {
  try {
    preflightResponse.value = await nginx_log.getPreflight(logPath.value)
    const range = preflightResponse.value.time_range
    if (preflightResponse.value.available && range) {
      timeRange.value = getInitialStructuredTimeRange(dayjs.unix(range.start), dayjs.unix(range.end))
      return true
    }
  }
  catch (error: unknown) {
    timeRange.value = getInitialStructuredTimeRange()
    throw error
  }

  timeRange.value = getInitialStructuredTimeRange()
  return false
}

// Start over for a newly opened log. A refresh keeps the results on screen
// until the new ones arrive.
async function initialize(refresh = false) {
  if (!refresh) {
    indexingStatus.value = 'indexing'
    hasSearched.value = false
    entries.value = []
    total.value = 0
  }
  currentPage.value = 1

  try {
    const wasReady = isReady.value
    if (await loadPreflight()) {
      indexingStatus.value = 'indexed'
      // A viewer that was ready searches through the time range watcher
      if (!wasReady)
        await performSearch()
    }
    // Otherwise wait for the index ready event
  }
  catch {
    indexingStatus.value = 'failed'
  }
}

async function handleIndexReadyNotification(data: { log_path: string }) {
  if (data.log_path !== (logPath.value || ''))
    return

  message.success($gettext('Log indexing completed! Loading updated data...'))
  await initialize(isReady.value)
}

onMounted(() => {
  indexReadySubscriptionId = websocketEventBus.subscribe('nginx_log_index_ready', data => {
    setTimeout(handleIndexReadyNotification, 1000, data)
  })
  initialize()
})

onUnmounted(() => {
  if (indexReadySubscriptionId)
    websocketEventBus.unsubscribe(indexReadySubscriptionId)
})

watch(logPath, () => {
  initialize()
})

watch(timeRange, () => {
  if (isReady.value)
    search()
}, { deep: true })

watch(levels, () => {
  if (isReady.value)
    search()
})
</script>

<template>
  <div>
    <div v-if="!isFailed">
      <div v-if="shouldShowControls" class="mb-4">
        <!-- Time Range Picker -->
        <div class="mb-4">
          <div class="mb-2 flex items-center justify-between gap-4">
            <div class="text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ $gettext('Time Range') }}
            </div>
            <div class="flex flex-wrap items-center justify-end gap-3">
              <slot name="time-range-right" />
            </div>
          </div>
          <ASpace wrap>
            <ADropdown placement="bottomLeft">
              <template #popupRender>
                <AMenu
                  :items="timePresetItems"
                  @click="({ key }) => applyTimePresetByKey(String(key))"
                />
              </template>
              <AButton>
                {{ $gettext('Quick Select') }}
                <DownOutlined />
              </AButton>
            </ADropdown>
            <RangePicker
              v-model:value="dateRange"
              show-time
              format="YYYY-MM-DD HH:mm:ss"
            />
            <AButton
              :loading="searchLoading"
              @click="performSearch"
            >
              <template #icon>
                <ReloadOutlined />
              </template>
            </AButton>
          </ASpace>
        </div>

        <!-- Search -->
        <div class="p-4 bg-gray-50 dark:bg-trueGray-800 rounded border border-gray-200 dark:border-trueGray-700">
          <div class="grid grid-cols-1 lg:grid-cols-4 gap-3">
            <div>
              <label class="block text-xs font-medium text-gray-700 dark:text-trueGray-300 mb-1">
                {{ $gettext('Level') }}
              </label>
              <ASelect
                v-model:value="levels"
                mode="multiple"
                :placeholder="$gettext('Any')"
                allow-clear
                class="w-full"
                :options="errorLevelOptions"
              />
            </div>
            <div class="lg:col-span-3">
              <label class="flex items-center gap-1 text-xs font-medium text-gray-700 dark:text-trueGray-300 mb-1">
                {{ $gettext('Full Text Search') }}
                <ATooltip placement="topLeft">
                  <template #title>
                    <div v-for="line in syntaxHelp" :key="line">
                      {{ line }}
                    </div>
                  </template>
                  <QuestionCircleOutlined class="text-gray-400 dark:text-trueGray-500 cursor-help" />
                </ATooltip>
              </label>
              <div class="flex gap-2">
                <AInput
                  v-model:value="query"
                  allow-clear
                  :placeholder="$gettext('Search, e.g. level:error upstream timed out -ip:10.0.0.1')"
                  @press-enter="search"
                />
                <AButton @click="resetSearch">
                  {{ $gettext('Reset') }}
                </AButton>
                <AButton type="primary" @click="search">
                  {{ $gettext('Search') }}
                </AButton>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Loading/Indexing State -->
      <LoadingState
        v-if="shouldShowIndexingSpinner || (searchLoading && !hasSearched)"
        :log-path="logPath || ''"
      />

      <div v-else-if="shouldShowResults">
        <div class="mb-3 text-sm text-gray-500 dark:text-gray-400">
          {{ $gettext('%{total} entries found in %{took} ms', {
            total: total.toLocaleString(),
            took: took.toLocaleString(),
          }) }}
        </div>

        <ATable
          v-if="entries.length > 0"
          :data-source="entries"
          :columns="columns"
          :loading="searchLoading"
          size="small"
          :scroll="{ x: 1400 }"
          :pagination="{
            current: currentPage,
            pageSize,
            total,
            showSizeChanger: {
              styles: {
                root: { minWidth: '100px' },
                content: { minWidth: '100px' },
                popup: {
                  listItem: { minWidth: '100px' },
                },
              },
            },
            showQuickJumper: true,
            pageSizeOptions: ['50', '100', '200', '500', '1000'],
            showTotal: (count, range) => $gettext('%{start}-%{end} of %{total} items', {
              start: range[0].toLocaleString(),
              end: range[1].toLocaleString(),
              total: count.toLocaleString(),
            }),
          }"
          @change="handleTableChange"
        >
          <template #bodyCell="{ column, record }">
            <span v-if="column.key === 'timestamp'" class="whitespace-nowrap">
              {{ dayjs.unix(record.timestamp).format('YYYY-MM-DD HH:mm:ss') }}
            </span>

            <ATag
              v-else-if="column.key === 'level'"
              :color="errorLevelColor(record.level)"
              class="m-0 uppercase"
            >
              {{ record.level || '-' }}
            </ATag>

            <div v-else-if="column.key === 'ip'" class="flex flex-wrap items-center gap-2">
              <ATag v-if="record.ip_location_label?.trim()" color="blue" class="m-0">
                {{ record.ip_location_label.trim() }}
              </ATag>
              <span v-if="record.ip" class="font-mono">{{ record.ip }}</span>
              <span v-else class="text-gray-400 dark:text-gray-500">-</span>
            </div>

            <template v-else-if="column.key === 'request'">
              <span v-if="errorLogRequest(record)" class="font-mono" :title="errorLogRequest(record)">
                {{ errorLogRequest(record) }}
              </span>
              <span v-else class="text-gray-400 dark:text-gray-500">-</span>
            </template>

            <div v-else-if="column.key === 'message'" class="min-w-0">
              <div class="break-words">
                {{ record.message || record.raw }}
              </div>
              <div
                v-if="errorLogContext(record).length > 0"
                class="mt-1 flex flex-wrap gap-x-4 gap-y-1 text-xs text-gray-500 dark:text-gray-400"
              >
                <span v-for="item in errorLogContext(record)" :key="item.key">
                  {{ item.key }}: <span class="font-mono">{{ item.value }}</span>
                </span>
              </div>
            </div>
          </template>

          <template #expandedRowRender="{ record }">
            <pre class="m-0 p-3 whitespace-pre-wrap break-all font-mono text-xs rounded bg-gray-50 dark:bg-trueGray-800 text-gray-800 dark:text-trueGray-200">{{ record.raw }}</pre>
          </template>
        </ATable>

        <div v-else class="text-center" style="padding: 40px;">
          <AEmpty
            :description="total > 0
              ? $gettext('No entries in current page')
              : $gettext('No logs found in the selected time range.')"
          />
          <p class="text-gray-500 dark:text-gray-400 mt-2">
            {{ $gettext('Try adjusting your search criteria or time range.') }}
          </p>
          <AButton v-if="query || levels.length > 0" class="mt-2" @click="resetSearch">
            {{ $gettext('Reset Search') }}
          </AButton>
        </div>
      </div>

      <!-- Empty State -->
      <div v-else class="text-center" style="padding: 40px;">
        <AEmpty :description="$gettext('No structured log data available')" />
        <div v-if="isReady" class="mt-4">
          <p class="text-gray-500 dark:text-gray-400">
            {{ $gettext('Try adjusting your search criteria or time range.') }}
          </p>
          <AButton type="primary" class="mt-2" @click="resetSearch">
            {{ $gettext('Reset Search') }}
          </AButton>
        </div>
      </div>
    </div>

    <!-- Failed State -->
    <div v-else class="text-center" style="padding: 80px 40px;">
      <AEmpty :description="$gettext('Log file not available')" />
    </div>
  </div>
</template>
