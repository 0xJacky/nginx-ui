<script setup lang="ts">
import type { SorterResult, TablePaginationConfig } from 'ant-design-vue/es/table/interface'
import type { AccessLogEntry, AdvancedSearchRequest, PreflightResponse } from '@/api/nginx_log'
import { DownOutlined, ExclamationCircleOutlined, ReloadOutlined } from '@ant-design/icons-vue'
import { Tag } from 'ant-design-vue'
import dayjs from 'dayjs'
import nginx_log from '@/api/nginx_log'
import { useWebSocketEventBus } from '@/composables/useWebSocketEventBus'
import { bytesToSize } from '@/lib/helper'
import LoadingState from '../components/LoadingState.vue'
import { useIndexProgress } from '../composables/useIndexProgress'
import SearchFilters from './components/SearchFilters.vue'

interface Props {
  logPath?: string
}

interface SearchSummary {
  pv?: number
  uv?: number
  total_traffic?: number
  unique_pages?: number
  avg_traffic_per_pv?: number
}

const props = defineProps<Props>()

const { message } = App.useApp()

// Route and router
const route = useRoute()

// WebSocket event bus for index ready notifications
const { subscribe: subscribeToEvent } = useWebSocketEventBus()

// Index progress tracking for this specific file
const { isFileIndexing } = useIndexProgress()

// Use provided log path or let backend determine default
const logPath = computed(() => props.logPath || undefined)

// Check if this is an error log (error logs don't support structured processing)
const isErrorLog = computed(() => {
  if (props.logPath) {
    return props.logPath.includes('error.log') || props.logPath.includes('error_log')
  }
  // Check route path
  return route.path.includes('error')
})

// Reactive data - Only advanced search mode now
const timeRange = ref({
  start: null as dayjs.Dayjs | null, // Will be set from server time range
  end: null as dayjs.Dayjs | null, // Will be set from server time range
})
const preflightResponse = ref<PreflightResponse | null>(null)
const searchFilters = ref({
  query: '',
  ip: '',
  method: '',
  status: [] as string[],
  path: '',
  user_agent: '',
  referer: '',
  browser: [] as string[],
  os: [] as string[],
  device: [] as string[],
})
const searchResults = ref<AccessLogEntry[]>([])
const searchTotal = ref(0)
const searchLoading = ref(false)
const indexingStatus = ref<'idle' | 'indexing' | 'indexed' | 'failed'>('idle')
const currentPage = ref(1)
const pageSize = ref(50)
const sortBy = ref<string>()
const sortOrder = ref<'asc' | 'desc'>('desc')
// Cache for failed path validations to prevent repeated calls
const pathValidationCache = ref<Map<string, boolean>>(new Map())
// Removed showAdvancedFilters - filters are always shown now

// Date range for ARangePicker
const dateRange = computed({
  get: () => {
    const start = timeRange.value.start
    const end = timeRange.value.end
    // Return undefined if either value is null/undefined, otherwise return the tuple
    return (start && end) ? [start, end] as [typeof start, typeof end] : undefined
  },
  set: value => {
    if (value && Array.isArray(value) && value.length === 2) {
      timeRange.value.start = value[0]
      timeRange.value.end = value[1]
    }
  },
})

const filteredEntries = computed(() => {
  // Since we removed the simple filter, just return search results directly
  return searchResults.value
})

// Summary stats from search response
const searchSummary = ref<SearchSummary | null>(null)

// Check if current file is being indexed (from WebSocket progress events)
const isCurrentFileIndexing = computed(() => {
  return logPath.value ? isFileIndexing(logPath.value) : false
})

// Check if preflight shows this specific file is available/indexed
const isFileAvailable = computed(() => {
  return preflightResponse.value?.available === true
})

// Computed properties for indexing status
const isLoading = computed(() => searchLoading.value)
const isReady = computed(() => indexingStatus.value === 'indexed')
const isFailed = computed(() => indexingStatus.value === 'failed')

// Combined status computed properties based on file-specific states
const shouldShowContent = computed(() => !isFailed.value)

const shouldShowControls = computed(() => {
  // Show controls when:
  // 1. File is available (indexed and ready) AND
  // 2. File is not currently being indexed
  return isFileAvailable.value && !isCurrentFileIndexing.value
})

const shouldShowIndexingSpinner = computed(() => {
  // Show indexing spinner if:
  // 1. Current file is actively being indexed (from WebSocket progress), OR
  // 2. Component is in indexing state but file is not yet available (waiting for initial index)
  return isCurrentFileIndexing.value || (indexingStatus.value === 'indexing' && !isFileAvailable.value)
})

const shouldShowResults = computed(() => {
  // Show results only when:
  // 1. File is available (indexed) AND
  // 2. File is not currently being re-indexed AND
  // 3. We have search results
  return isFileAvailable.value && !isCurrentFileIndexing.value && searchSummary.value !== null
})

// Status code color mapping
function getStatusColor(status: number): string {
  if (status >= 200 && status < 300)
    return 'success'
  if (status >= 300 && status < 400)
    return 'processing'
  if (status >= 400 && status < 500)
    return 'warning'
  if (status >= 500)
    return 'error'
  return 'default'
}

// Device type color mapping
function getDeviceColor(deviceType: string): string {
  const colors: Record<string, string> = {
    'Desktop': 'blue',
    'Mobile': 'green',
    'Tablet': 'orange',
    'Bot': 'red',
    'TV': 'purple',
    'Smart Speaker': 'cyan',
    'Game Console': 'magenta',
    'Wearable': 'gold',
  }
  return colors[deviceType] || 'default'
}

// Get sort order for column
function getSortOrder(fieldName: string): 'ascend' | 'descend' | undefined {
  if (sortBy.value === fieldName) {
    return sortOrder.value === 'asc' ? 'ascend' : 'descend'
  }
  return undefined
}

// Table columns configuration
const structuredLogColumns = computed(() => [
  {
    title: $gettext('Time'),
    dataIndex: 'timestamp',
    width: 140,
    fixed: 'left' as const,
    sorter: true,
    sortOrder: getSortOrder('timestamp'),
    customRender: ({ record }: { record: AccessLogEntry }) => h('span', dayjs.unix(record.timestamp).format('YYYY-MM-DD HH:mm:ss')),
  },
  {
    title: $gettext('IP'),
    dataIndex: 'ip',
    width: 350,
    sorter: true,
    sortOrder: getSortOrder('ip'),
    customRender: ({ record }: { record: AccessLogEntry }) => {
      const locationParts: string[] = []
      if (record.region_code) {
        locationParts.push(record.region_code)
      }
      if (record.province) {
        locationParts.push(record.province)
      }
      if (record.city) {
        locationParts.push(record.city)
      }

      return h('div', { class: 'flex items-center gap-2' }, [
        locationParts.length > 0 ? h(Tag, { color: 'blue', size: 'small' }, { default: () => locationParts.join(' · ') }) : null,
        h('span', record.ip),
      ])
    },
  },
  {
    title: $gettext('Request'),
    dataIndex: 'path',
    ellipsis: {
      showTitle: true,
    },
    width: 350,
    customRender: ({ record }: { record: AccessLogEntry }) => {
      let methodColor = 'default'
      if (record.method === 'GET')
        methodColor = 'green'
      else if (record.method === 'POST')
        methodColor = 'blue'

      return h('div', [
        h(Tag, {
          color: methodColor,
          size: 'small',
        }, { default: () => record.method }),
        h('span', { class: 'ml-1' }, record.path),
      ])
    },
  },
  {
    title: $gettext('Status'),
    dataIndex: 'status',
    width: 80,
    sorter: true,
    sortOrder: getSortOrder('status'),
    customRender: ({ record }: { record: AccessLogEntry }) => h(Tag, { color: getStatusColor(record.status) }, { default: () => record.status }),
  },
  {
    title: $gettext('Size'),
    dataIndex: 'bytes_sent',
    width: 80,
    sorter: true,
    sortOrder: getSortOrder('bytes_sent'),
    customRender: ({ record }: { record: AccessLogEntry }) => h('span', bytesToSize(record.bytes_sent)),
  },
  {
    title: $gettext('Browser'),
    dataIndex: 'browser',
    width: 120,
    sorter: true,
    sortOrder: getSortOrder('browser'),
    customRender: ({ record }: { record: AccessLogEntry }) => {
      if (record.browser && record.browser !== 'Unknown') {
        const browserText = record.browser_version
          ? `${record.browser} ${record.browser_version}`
          : record.browser
        return h('div', browserText)
      }
      return null
    },
  },
  {
    title: $gettext('OS'),
    dataIndex: 'os',
    width: 120,
    sorter: true,
    sortOrder: getSortOrder('os'),
    customRender: ({ record }: { record: AccessLogEntry }) => {
      if (record.os && record.os !== 'Unknown') {
        const osText = record.os_version
          ? `${record.os} ${record.os_version}`
          : record.os
        return h('div', osText)
      }
      return null
    },
  },
  {
    title: $gettext('Device'),
    dataIndex: 'device_type',
    width: 90,
    sorter: true,
    sortOrder: getSortOrder('device_type'),
    customRender: ({ record }: { record: AccessLogEntry }) => record.device_type
      ? h(Tag, { color: getDeviceColor(record.device_type), size: 'small' }, { default: () => record.device_type })
      : null,
  },
  {
    title: $gettext('Referer'),
    dataIndex: 'referer',
    ellipsis: true,
    width: 200,
    customRender: ({ record }: { record: AccessLogEntry }) => record.referer && record.referer !== '-'
      ? h('span', record.referer)
      : null,
  },
])

// Time range presets (Grafana-style)
const timePresets = [
  { label: () => $gettext('Last 15 minutes'), value: () => ({ start: dayjs().subtract(15, 'minute'), end: dayjs() }) },
  { label: () => $gettext('Last 30 minutes'), value: () => ({ start: dayjs().subtract(30, 'minute'), end: dayjs() }) },
  { label: () => $gettext('Last hour'), value: () => ({ start: dayjs().subtract(1, 'hour'), end: dayjs() }) },
  { label: () => $gettext('Last 4 hours'), value: () => ({ start: dayjs().subtract(4, 'hour'), end: dayjs() }) },
  { label: () => $gettext('Last 12 hours'), value: () => ({ start: dayjs().subtract(12, 'hour'), end: dayjs() }) },
  { label: () => $gettext('Last 24 hours'), value: () => ({ start: dayjs().subtract(24, 'hour'), end: dayjs() }) },
  { label: () => $gettext('Last 7 days'), value: () => ({ start: dayjs().subtract(7, 'day'), end: dayjs() }) },
  { label: () => $gettext('Last 30 days'), value: () => ({ start: dayjs().subtract(30, 'day'), end: dayjs() }) },
]

// Load structured logs function - now only uses advanced search
async function loadLogs() {
  await performAdvancedSearch()
}

// Advanced search function
async function performAdvancedSearch() {
  // Don't search if time range is not set yet
  if (!timeRange.value.start || !timeRange.value.end) {
    return
  }

  searchLoading.value = true
  try {
    const searchRequest: AdvancedSearchRequest = {
      start_time: timeRange.value.start.unix(),
      end_time: timeRange.value.end.unix(),
      query: searchFilters.value.query || undefined,
      ip: searchFilters.value.ip || undefined,
      method: searchFilters.value.method || undefined,
      status: searchFilters.value.status.length > 0 ? searchFilters.value.status.map(s => Number.parseInt(s)).filter(n => !Number.isNaN(n)) : undefined,
      path: searchFilters.value.path || undefined,
      user_agent: searchFilters.value.user_agent || undefined,
      referer: searchFilters.value.referer || undefined,
      browser: searchFilters.value.browser.length > 0 ? searchFilters.value.browser.join(',') : undefined,
      os: searchFilters.value.os.length > 0 ? searchFilters.value.os.join(',') : undefined,
      device: searchFilters.value.device.length > 0 ? searchFilters.value.device.join(',') : undefined,
      limit: pageSize.value,
      offset: (currentPage.value - 1) * pageSize.value,
      sort_by: sortBy.value,
      sort_order: sortOrder.value,
      log_path: logPath.value,
    }

    const result = await nginx_log.search(searchRequest)

    searchResults.value = result.entries || []
    searchTotal.value = result.total || 0
    searchSummary.value = result.summary || null
  }
  catch (error: unknown) {
    // Check if this is a path validation error - don't show message for these
    if (isPathValidationError(error)) {
      // Silently reset results for path validation errors
      searchResults.value = []
      searchTotal.value = 0
      return
    }

    // Reset results on error
    searchResults.value = []
    searchTotal.value = 0
    searchSummary.value = null
  }
  finally {
    searchLoading.value = false
  }
}

// Load preflight information (single request, no retries)
async function loadPreflight(): Promise<boolean> {
  // Check cache for known invalid paths
  const currentPath = logPath.value || ''
  if (pathValidationCache.value.has(currentPath) && !pathValidationCache.value.get(currentPath)) {
    throw new Error('Path validation failed (cached)')
  }

  try {
    preflightResponse.value = await nginx_log.getPreflight(logPath.value)

    if (preflightResponse.value.available && preflightResponse.value.time_range) {
      // Cache this path as valid and set time range
      pathValidationCache.value.set(currentPath, true)
      // Set time range to full days: start_date 00:00:00 to end_date 23:59:59
      const startTime = dayjs.unix(preflightResponse.value.time_range.start).startOf('day')
      const endTime = dayjs.unix(preflightResponse.value.time_range.end).endOf('day')

      timeRange.value.start = startTime
      timeRange.value.end = endTime
      return true // Index is ready
    }
    else {
      // Index is not ready, will wait for event notification
      // Don't show message here - let the UI status handle it
      // Use default range temporarily
      timeRange.value.start = dayjs().subtract(7, 'day')
      timeRange.value.end = dayjs()
      return false // Index not ready
    }
  }
  catch (error: unknown) {
    // Check if this is a path validation error by error code
    if (isPathValidationError(error)) {
      // Cache this path as invalid to prevent future calls
      pathValidationCache.value.set(currentPath, false)
      throw error // Immediately fail for path validation errors
    }

    // For other errors, set fallback range but don't show error message here
    // The error will be handled by the caller
    timeRange.value.start = dayjs().subtract(7, 'day')
    timeRange.value.end = dayjs()
    throw error // Let the caller handle the error message
  }
}

// Apply time preset
function applyTimePreset(preset: { value: () => { start: dayjs.Dayjs, end: dayjs.Dayjs } }) {
  const range = preset.value()
  timeRange.value = range
  loadLogs()
}

// Reset search filters
function resetSearchFilters() {
  searchFilters.value = {
    query: '',
    ip: '',
    method: '',
    status: [],
    path: '',
    user_agent: '',
    referer: '',
    browser: [],
    os: [],
    device: [],
  }
  currentPage.value = 1
  performAdvancedSearch()
}

// Note: handleSortingChange function removed - sorting is now handled directly in handleTableChange

// Handle table sorting and pagination change
function handleTableChange(
  pagination: TablePaginationConfig,
  filters: Record<string, unknown>,
  sorter: SorterResult<AccessLogEntry> | SorterResult<AccessLogEntry>[],
) {
  let shouldResetPage = false

  // Update page size first
  if (pagination.pageSize !== undefined && pagination.pageSize !== pageSize.value) {
    pageSize.value = pagination.pageSize
    shouldResetPage = true // Reset to first page when page size changes
  }

  // Handle sorting changes
  const singleSorter = Array.isArray(sorter) ? sorter[0] : sorter

  if (singleSorter?.field) {
    const newSortBy = mapColumnToSortField(String(singleSorter.field))
    // When order is not present, it means to clear sorting, so we revert to default
    const newSortOrder = singleSorter.order === 'ascend' ? 'asc' : 'desc'
    const newSortField = singleSorter.order ? newSortBy : undefined

    // Check if sorting actually changed
    if (newSortField !== sortBy.value || newSortOrder !== sortOrder.value) {
      sortBy.value = newSortField
      sortOrder.value = newSortOrder
      shouldResetPage = true // Reset to first page when sorting changes
    }
  }

  // Update pagination (do this after handling sort/pageSize)
  if (shouldResetPage) {
    currentPage.value = 1
  }
  else if (pagination.current !== undefined) {
    currentPage.value = pagination.current
  }

  nextTick(() => {
    performAdvancedSearch()
  })
}

// Map table column names to backend sort fields
function mapColumnToSortField(column: string): string {
  const mapping: Record<string, string> = {
    timestamp: 'timestamp',
    ip: 'ip',
    method: 'method',
    path: 'path',
    status: 'status',
    bytes_sent: 'bytes_sent',
    browser: 'browser',
    os: 'os',
    device_type: 'device_type',
  }
  return mapping[column] || 'timestamp'
}

// Get display name for sort field
function getSortDisplayName(field: string): string {
  const displayNames: Record<string, string> = {
    timestamp: $gettext('Time'),
    ip: $gettext('IP Address'),
    method: $gettext('Method'),
    path: $gettext('Path'),
    status: $gettext('Status'),
    bytes_sent: $gettext('Size'),
    browser: $gettext('Browser'),
    os: $gettext('OS'),
    device_type: $gettext('Device'),
  }
  return displayNames[field] || field
}

// Reset sorting to default
function resetSorting() {
  sortBy.value = 'timestamp'
  sortOrder.value = 'desc'
  currentPage.value = 1
  performAdvancedSearch()
}

// Helper function to check if error is a path validation error
function isPathValidationError(error: unknown): boolean {
  if (!(error instanceof Error)) {
    return false
  }

  try {
    // Check if error response contains path validation error codes
    const errorData = JSON.parse(error.message)
    if (errorData.scope === 'nginx_log' && errorData.code) {
      const code = Number.parseInt(errorData.code)
      // Path validation error codes: 50013, 50014, 50015
      return code === 50013 || code === 50014 || code === 50015
    }
  }
  catch {
    // Ignore parsing errors
  }

  return false
}

// Handle initialization with indexed data and search
async function handleInitializedData(hasIndexedData: boolean) {
  if (timeRange.value.start && timeRange.value.end) {
    await performAdvancedSearch()

    // Only show messages for specific scenarios
    if (searchResults.value.length === 0 && hasIndexedData) {
      message.info($gettext('No logs found in the selected time range.'))
    }
    else if (searchResults.value.length > 0 && !hasIndexedData) {
      message.info($gettext('Background indexing in progress. Data will be updated automatically when ready.'))
    }
  }
}

// Handle index ready notification from WebSocket
async function handleIndexReadyNotification(data: {
  log_path: string
  start_time: number
  end_time: number
  available: boolean
  index_status: string
}) {
  const currentPath = logPath.value || ''
  // Check if the notification is for the current log path
  if (data.log_path === currentPath) {
    message.success($gettext('Log indexing completed! Loading updated data...'))

    try {
      // Re-request preflight to get the latest information
      const hasIndexedData = await loadPreflight()

      if (hasIndexedData) {
        indexingStatus.value = 'indexed'
        // Load initial data with the updated time range
        await performAdvancedSearch()
      }
    }
    catch (error) {
      console.error('Failed to reload preflight after indexing completion:', error)
      indexingStatus.value = 'failed'
    }
  }
}

// Initialize on mount
onMounted(async () => {
  // Skip initialization for error logs
  if (isErrorLog.value) {
    return
  }

  // Subscribe to index ready notifications
  subscribeToEvent('nginx_log_index_ready', data => {
    setTimeout(() => handleIndexReadyNotification(data), 1000)
  })

  indexingStatus.value = 'indexing'

  try {
    const hasIndexedData = await loadPreflight()

    if (hasIndexedData) {
      // Index is ready and data is available
      indexingStatus.value = 'indexed'
      await handleInitializedData(hasIndexedData)
    }

    // Index is not ready yet, keep indexing status and wait for event notification
    // indexingStatus remains 'indexing'
  }
  catch {
    indexingStatus.value = 'failed'
    // Don't show any error messages - the empty page clearly indicates the issue
  }
})

// Watch for log path changes to clear cache and reload
watch(logPath, (newPath, oldPath) => {
  // Clear cache when path changes
  if (newPath !== oldPath) {
    pathValidationCache.value.clear()
  }
  if (isReady.value) {
    loadLogs()
  }
})

// Watch for time range changes (only after initialization)
watch(timeRange, () => {
  if (isReady.value) {
    loadLogs()
  }
}, { deep: true })
</script>

<template>
  <div>
    <!-- Error Log Notice -->
    <div v-if="isErrorLog" class="mb-6 p-4 bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-200 dark:border-yellow-800 rounded">
      <div class="flex items-start">
        <div class="flex-shrink-0">
          <ExclamationCircleOutlined class="h-5 w-5 text-yellow-400" />
        </div>
        <div class="ml-3">
          <h3 class="text-sm font-medium text-yellow-800 dark:text-yellow-200">
            {{ $gettext('Error Log Detected') }}
          </h3>
          <div class="mt-2 text-sm text-yellow-700 dark:text-yellow-300">
            <p>{{ $gettext('Error logs do not support structured analysis as they contain free-form text messages.') }}</p>
            <p class="mt-1">
              {{ $gettext('For error logs, please use the Raw Log Viewer for better viewing experience.') }}
            </p>
          </div>
          <div class="mt-3">
            <AButton size="small" type="primary" @click="$router.push('/nginx_log')">
              {{ $gettext('Go to Raw Log Viewer') }}
            </AButton>
          </div>
        </div>
      </div>
    </div>

    <!-- Access Log Content (only show for non-error logs and non-failed preflight) -->
    <div v-else-if="shouldShowContent">
      <!-- Time Range and Search Controls (only show when ready) -->
      <div v-if="shouldShowControls" class="mb-4">
        <!-- Time Range Picker -->
        <div class="mb-4">
          <div class="mb-2 text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ $gettext('Time Range') }}
          </div>
          <ASpace wrap>
            <ADropdown placement="bottomLeft">
              <template #overlay>
                <AMenu @click="({ key }) => applyTimePreset(timePresets[Number(key)])">
                  <AMenuItem v-for="(preset, index) in timePresets" :key="index">
                    {{ preset.label() }}
                  </AMenuItem>
                </AMenu>
              </template>
              <AButton>
                {{ $gettext('Quick Select') }}
                <DownOutlined />
              </AButton>
            </ADropdown>
            <ARangePicker
              v-model:value="dateRange"
              show-time
              format="YYYY-MM-DD HH:mm:ss"
              @change="performAdvancedSearch"
            />
            <AButton
              type="default"
              :loading="isCurrentFileIndexing || !isFileAvailable"
              :disabled="isCurrentFileIndexing || !isFileAvailable"
              @click="loadLogs"
            >
              <template #icon>
                <ReloadOutlined />
              </template>
            </AButton>
          </ASpace>
        </div>

        <!-- Search Filters -->
        <SearchFilters
          v-model="searchFilters"
          class="mb-6"
          @search="performAdvancedSearch"
          @reset="resetSearchFilters"
        />

        <!-- Sort Info -->
        <div v-if="sortBy" class="mb-4 p-2 bg-blue-50 dark:bg-blue-900/20 rounded border border-blue-200 dark:border-blue-800">
          <span class="text-sm text-blue-600 dark:text-blue-300">
            {{ $gettext('Sorted by') }}: <strong>{{ getSortDisplayName(sortBy) }}</strong> ({{ sortOrder === 'asc' ? $gettext('Ascending') : $gettext('Descending') }})
          </span>
          <AButton size="small" type="text" class="ml-2" @click="resetSorting">
            {{ $gettext('Reset') }}
          </AButton>
        </div>
      </div>

      <!-- Loading/Indexing State -->
      <LoadingState
        v-if="isLoading || shouldShowIndexingSpinner"
        :log-path="logPath || ''"
      />

      <!-- Search Results (show when indexing is ready and we have search results) -->
      <div v-else-if="shouldShowResults">
        <!-- Summary -->
        <div class="mb-4 p-4 bg-gray-50 dark:bg-trueGray-800 rounded">
          <div class="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-6 gap-4">
            <div class="text-center">
              <AStatistic
                :title="$gettext('Total Entries')"
                :value="searchTotal"
              />
            </div>
            <div class="text-center">
              <AStatistic
                :title="$gettext('PV')"
                :value="searchSummary?.pv || 0"
              />
            </div>
            <div class="text-center">
              <AStatistic
                :title="$gettext('UV')"
                :value="searchSummary?.uv || 0"
              />
            </div>
            <div class="text-center">
              <AStatistic
                :title="$gettext('Traffic')"
                :value="bytesToSize(searchSummary?.total_traffic || 0)"
              />
            </div>
            <div class="text-center">
              <AStatistic
                :title="$gettext('Unique Pages')"
                :value="searchSummary?.unique_pages || 0"
              />
            </div>
            <div class="text-center">
              <AStatistic
                :title="$gettext('Avg/PV')"
                :value="bytesToSize(Math.round(searchSummary?.avg_traffic_per_pv || 0))"
              />
            </div>
          </div>
        </div>

        <!-- Log Table (show if we have entries) -->
        <div v-if="filteredEntries.length > 0" class="log-table-container">
          <ATable
            :data-source="filteredEntries"
            :pagination="{
              current: currentPage,
              pageSize,
              total: searchTotal,
              showSizeChanger: true,
              showQuickJumper: true,
              pageSizeOptions: ['50', '100', '200', '500', '1000'],
              showTotal: (total, range) => $gettext('%{start}-%{end} of %{total} items', {
                start: range[0].toLocaleString(),
                end: range[1].toLocaleString(),
                total: total.toLocaleString(),
              }),
            }"
            size="small"
            :scroll="{ x: 2400 }"
            :columns="structuredLogColumns"
            :loading="isLoading"
            @change="handleTableChange"
          />
        </div>

        <!-- Empty State within search results -->
        <div v-else class="text-center" style="padding: 40px;">
          <AEmpty :description="$gettext('No entries in current page')" />
          <p class="text-gray-500 mt-2">
            {{ $gettext('Try adjusting your search criteria or navigate to different pages.') }}
          </p>
        </div>
      </div>

      <!-- Empty State -->
      <div v-else class="text-center" style="padding: 40px;">
        <AEmpty :description="$gettext('No structured log data available')" />
        <div v-if="isReady" class="mt-4">
          <p class="text-gray-500">
            {{ $gettext('Try adjusting your search criteria or time range.') }}
          </p>
          <p v-if="timeRange.start && timeRange.end" class="text-gray-400 text-sm mt-2">
            {{ $gettext('Search range') }}: {{ timeRange.start.format('YYYY-MM-DD HH:mm') }} - {{ timeRange.end.format('YYYY-MM-DD HH:mm') }}
            <Tag v-if="preflightResponse && preflightResponse.available" color="green" size="small" class="ml-2">
              {{ $gettext('From indexed logs') }}
            </Tag>
            <Tag v-else color="orange" size="small" class="ml-2">
              {{ $gettext('Default range') }}
            </Tag>
          </p>
          <AButton type="primary" class="mt-2" @click="resetSearchFilters">
            {{ $gettext('Reset Search') }}
          </AButton>
        </div>
      </div>
    </div> <!-- End of Access Log Content -->

    <!-- Failed State (show empty page when preflight fails) -->
    <div v-else class="text-center" style="padding: 80px 40px;">
      <AEmpty :description="$gettext('Log file not available')" />
    </div>
  </div>
</template>

<style scoped>
/* Fix pagination page size selector width */
:deep(.log-table-container .ant-pagination-options-size-changer .ant-select) {
  min-width: 100px !important;
}

:deep(.log-table-container .ant-pagination-options-size-changer .ant-select-selector) {
  min-width: 100px !important;
}

/* Ensure the dropdown has enough width */
:deep(.ant-select-dropdown .ant-select-item) {
  min-width: 100px;
}
</style>
