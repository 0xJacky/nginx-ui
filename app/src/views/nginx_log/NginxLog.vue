<script setup lang="ts">
import { useRouteQuery } from '@vueuse/router'
import nginxLog from '@/api/nginx_log'
import FooterToolBar from '@/components/FooterToolbar'
import DashboardViewer from './dashboard/DashboardViewer.vue'
import RawLogViewer from './raw/RawLogViewer.vue'
import StructuredLogViewer from './structured/StructuredLogViewer.vue'

// Route and router
const route = useRoute()
const router = useRouter()

// Setup log control data based on route params
const logPath = computed(() => route.query.path?.toString() ?? '')
const logType = computed(() => {
  if (route.path.indexOf('access') > 0)
    return 'access'
  return route.path.indexOf('error') > 0 ? 'error' : 'site'
})

const viewMode = useRouteQuery<'raw' | 'structured' | 'dashboard'>('view', 'structured')

interface LogPathOption {
  label: string
  value: string
}

const logPathOptions = ref<LogPathOption[]>([])
const isLoadingLogPathOptions = ref(false)

const maxLogPathLength = computed(() => {
  const optionMax = logPathOptions.value.reduce((max, option) => {
    return Math.max(max, option.label.length)
  }, 0)
  return Math.max(optionMax, logPath.value.length, 20)
})

const logSelectStyle = computed(() => ({
  width: `min(calc(100vw - 4rem), ${maxLogPathLength.value + 8}ch)`,
  minWidth: '20rem',
}))

const logSelectDropdownStyle = computed(() => ({
  width: `min(calc(100vw - 2rem), ${maxLogPathLength.value + 12}ch)`,
  minWidth: '20rem',
}))

const selectedLogPath = computed({
  get: () => logPath.value,
  set: (value: string) => {
    const query = { ...route.query }
    if (value) {
      query.path = value
    }
    else {
      delete query.path
    }
    router.replace({ query })
  },
})

function normalizePath(path: string) {
  return path.replace(/\\/g, '/')
}

function ensureCurrentPathOption(path: string) {
  if (!path)
    return

  const normalizedCurrent = normalizePath(path)
  const exists = logPathOptions.value.some(option => normalizePath(option.value) === normalizedCurrent)
  if (!exists) {
    logPathOptions.value.unshift({
      label: path,
      value: path,
    })
  }
}

async function fetchLogPathOptions() {
  if (logType.value !== 'access' && logType.value !== 'error') {
    logPathOptions.value = []
    return
  }

  isLoadingLogPathOptions.value = true
  try {
    const res = await nginxLog.list({ type: logType.value })
    const logs = Array.isArray(res?.data) ? res.data : []

    logPathOptions.value = logs
      .filter(item => !!item.path)
      .map(item => ({
        label: item.path!,
        value: item.path!,
      }))

    ensureCurrentPathOption(logPath.value)

    if (!selectedLogPath.value && logPathOptions.value.length > 0) {
      selectedLogPath.value = logPathOptions.value[0].value
    }
  }
  catch (err) {
    console.error('Failed to load log path options:', err)
    ensureCurrentPathOption(logPath.value)
  }
  finally {
    isLoadingLogPathOptions.value = false
  }
}

// Indexing status
const isIndexingEnabled = ref(false)

onMounted(async () => {
  try {
    const res = await nginxLog.getAdvancedIndexingStatus()
    isIndexingEnabled.value = !!res.enabled
  }
  catch (err) {
    console.error('Failed to get indexing status:', err)
    isIndexingEnabled.value = false
  }

  await fetchLogPathOptions()
})

watch(logType, async () => {
  await fetchLogPathOptions()
})

watch(logPath, newPath => {
  ensureCurrentPathOption(newPath)
})

// Check if this is an error log
const isErrorLog = computed(() => {
  return logType.value === 'error' || logPath.value.includes('error.log') || logPath.value.includes('error_log')
})

const autoRefresh = ref(true)

watch(viewMode, v => {
  if (v === 'structured' && (!isIndexingEnabled.value || isErrorLog.value)) {
    viewMode.value = 'raw'
  }
}, { immediate: true })

// View mode logic: set defaults based on log type and indexing status
watch([isErrorLog, isIndexingEnabled], ([isError, enabled], [prevIsError, prevEnabled]) => {
  // Only set default when conditions change or initial load
  const isInitialLoad = prevIsError === undefined && prevEnabled === undefined
  const conditionsChanged = isError !== prevIsError || enabled !== prevEnabled

  if (isInitialLoad || conditionsChanged) {
    if (isError) {
      // Error logs always use raw mode
      viewMode.value = 'raw'
    }
    else if (!enabled) {
      // Indexing disabled: default to raw
      viewMode.value = 'raw'
    }
    else if (enabled && logType.value === 'access') {
      // Indexing enabled for access logs: default to structured
      viewMode.value = 'structured'
    }
  }
}, { immediate: true })
</script>

<template>
  <ACard
    :title="$gettext('Nginx Log')"
    variant="borderless"
  >
    <template #extra>
      <div class="flex flex-wrap items-center justify-end gap-4">
        <ASelect
          v-model:value="selectedLogPath"
          class="flex-none font-mono"
          :style="logSelectStyle"
          :dropdown-style="logSelectDropdownStyle"
          show-search
          allow-clear
          :loading="isLoadingLogPathOptions"
          :options="logPathOptions"
          :placeholder="$gettext('Select log file')"
          :filter-option="(input, option) =>
            ((option?.label ?? '') as string).toLowerCase().includes(input.toLowerCase())"
        />

        <!-- View Mode Toggle (hide for error logs or when indexing is disabled) -->
        <div v-if="!isErrorLog && isIndexingEnabled" class="flex items-center">
          <ASegmented
            v-model:value="viewMode"
            :options="[
              { label: $gettext('Structured'), value: 'structured' },
              { label: $gettext('Dashboard'), value: 'dashboard' },
              { label: $gettext('Raw'), value: 'raw' },
            ]"
          />
        </div>

        <!-- Auto Refresh (only for raw mode) -->
        <div v-if="viewMode === 'raw'" class="flex items-center">
          <span class="mr-2">{{ $gettext('Auto Refresh') }}</span>
          <ASwitch v-model:checked="autoRefresh" />
        </div>
      </div>
    </template>

    <!-- Raw Log View -->
    <RawLogViewer
      v-if="viewMode === 'raw'"
      :log-path="logPath"
      :log-type="logType"
      :auto-refresh="autoRefresh"
    />

    <!-- Structured Log View -->
    <StructuredLogViewer
      v-else-if="viewMode === 'structured'"
      :log-path="logPath"
    />

    <!-- Dashboard View -->
    <DashboardViewer
      v-else-if="viewMode === 'dashboard'"
      :log-path="logPath"
    />

    <FooterToolBar v-if="logPath">
      <AButton @click="router.go(-1)">
        {{ $gettext('Back') }}
      </AButton>
    </FooterToolBar>
  </ACard>
</template>
