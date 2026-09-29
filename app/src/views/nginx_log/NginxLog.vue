<script setup lang="ts">
import { useRouteQuery } from '@vueuse/router'
import nginxLog from '@/api/nginx_log'
import FooterToolBar from '@/components/FooterToolbar'
import PluginSlotItem from '@/components/PluginSlot/PluginSlotItem.vue'
import { NGINX_LOG_VIEW_SLOT_PREFIX, uniqueByKey } from '@/plugin/slots'
import { usePluginStore } from '@/plugin/store'
import DashboardViewer from './dashboard/DashboardViewer.vue'
import RawLogViewer from './raw/RawLogViewer.vue'
import StructuredLogViewer from './structured/StructuredLogViewer.vue'

// Route and router
const route = useRoute()
const router = useRouter()

// Setup log control data based on route params
const logPath = computed(() => route.query.path?.toString() ?? '')
const logType = computed(() => {
  const queryType = route.query.type?.toString().toLowerCase() ?? ''
  if (queryType === 'access' || queryType === 'error')
    return queryType

  const pathType = route.path.split('/').filter(Boolean).pop()?.toLowerCase() ?? ''
  if (pathType === 'access' || pathType === 'error' || pathType === 'site')
    return pathType

  return 'site'
})

// A built-in mode or the key of a view a plugin registered
const viewMode = useRouteQuery<string>('view', 'structured')

const BUILTIN_VIEW_MODES = ['raw', 'structured', 'dashboard']

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

const logSelectStyles = computed(() => ({
  popup: {
    root: {
      width: `min(calc(100vw - 2rem), ${maxLogPathLength.value + 12}ch)`,
      minWidth: '20rem',
    },
  },
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
const isIndexingEnabled = ref<boolean | null>(null)

onMounted(async () => {
  try {
    const res = await nginxLog.getAdvancedIndexingStatus()
    isIndexingEnabled.value = res.enabled
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
  if (logType.value === 'error')
    return true

  if (logType.value === 'access')
    return false

  return logPath.value.includes('error.log') || logPath.value.includes('error_log')
})

const autoRefresh = ref(true)

// Views plugins add after the built-in modes. A key that equals a built-in
// mode is ignored.
const pluginStore = usePluginStore()

const pluginViewContext = computed(() => ({
  path: logPath.value,
  type: isErrorLog.value ? 'error' : 'access',
}))

const pluginViews = computed(() => uniqueByKey(pluginStore.slotsByPrefix(NGINX_LOG_VIEW_SLOT_PREFIX, pluginViewContext.value))
  .filter(item => !BUILTIN_VIEW_MODES.includes(item.key)))

const activePluginView = computed(() => pluginViews.value.find(item => item.key === viewMode.value))

// What the page shows for the query value. An unknown value shows the raw
// view once plugins finished loading, until then nothing is decided yet.
const effectiveView = computed(() => {
  if (BUILTIN_VIEW_MODES.includes(viewMode.value) || activePluginView.value)
    return viewMode.value

  return pluginStore.ready ? 'raw' : ''
})

const viewModeOptions = computed(() => {
  const advancedViewDisabled = isIndexingEnabled.value !== true

  // Error logs only have the raw view of their own.
  const builtin = isErrorLog.value
    ? [{ label: $gettext('Raw'), value: 'raw' }]
    : [
        { label: $gettext('Structured'), value: 'structured', disabled: advancedViewDisabled },
        { label: $gettext('Dashboard'), value: 'dashboard', disabled: advancedViewDisabled },
        { label: $gettext('Raw'), value: 'raw' },
      ]

  return [
    ...builtin,
    ...pluginViews.value.map(item => ({
      label: $gettext(item.registration.label ?? item.key),
      value: item.key,
    })),
  ]
})

const segmentedValue = computed({
  get: () => effectiveView.value || viewMode.value,
  set: (value: string) => {
    viewMode.value = value
  },
})

const showViewToggle = computed(() => !isErrorLog.value || pluginViews.value.length > 0)

watch(viewMode, v => {
  if (v === 'structured' && (isIndexingEnabled.value === false || isErrorLog.value)) {
    viewMode.value = 'raw'
  }
}, { immediate: true })

// View mode logic: set defaults based on log type and indexing status
watch([isErrorLog, isIndexingEnabled], ([isError, enabled], [prevIsError, prevEnabled]) => {
  if (enabled === null)
    return

  // A plugin view, or a link plugins have not resolved yet, stays as it is.
  if (!BUILTIN_VIEW_MODES.includes(viewMode.value) && (!pluginStore.ready || activePluginView.value))
    return

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
    <div v-if="effectiveView !== 'structured'" class="mb-4 flex flex-wrap items-center justify-end gap-4">
      <ASelect
        v-model:value="selectedLogPath"
        class="flex-none font-mono"
        :style="logSelectStyle"
        :styles="logSelectStyles"
        show-search
        allow-clear
        :loading="isLoadingLogPathOptions"
        :options="logPathOptions"
        :placeholder="$gettext('Select log file')"
        :filter-option="(input, option) =>
          ((option?.label ?? '') as string).toLowerCase().includes(input.toLowerCase())"
      />

      <!-- View Mode Toggle (error logs only have it when a plugin adds a view) -->
      <div v-if="showViewToggle" class="flex items-center">
        <ASegmented
          v-model:value="segmentedValue"
          :options="viewModeOptions"
        />
      </div>

      <!-- Auto Refresh (only for raw mode) -->
      <div v-if="effectiveView === 'raw'" class="flex items-center">
        <span class="mr-2">{{ $gettext('Auto Refresh') }}</span>
        <ASwitch v-model:checked="autoRefresh" />
      </div>
    </div>

    <!-- Raw Log View -->
    <RawLogViewer
      v-if="effectiveView === 'raw'"
      :log-path="logPath"
      :log-type="logType"
      :auto-refresh="autoRefresh"
    />

    <!-- Structured Log View -->
    <StructuredLogViewer
      v-else-if="effectiveView === 'structured'"
      :log-path="logPath"
    >
      <template #time-range-right>
        <ASelect
          v-model:value="selectedLogPath"
          class="flex-none font-mono"
          :style="logSelectStyle"
          :styles="logSelectStyles"
          show-search
          allow-clear
          :loading="isLoadingLogPathOptions"
          :options="logPathOptions"
          :placeholder="$gettext('Select log file')"
          :filter-option="(input, option) =>
            ((option?.label ?? '') as string).toLowerCase().includes(input.toLowerCase())"
        />
        <div v-if="showViewToggle" class="flex items-center">
          <ASegmented
            v-model:value="segmentedValue"
            :options="viewModeOptions"
          />
        </div>
      </template>
    </StructuredLogViewer>

    <!-- Dashboard View -->
    <DashboardViewer
      v-else-if="effectiveView === 'dashboard'"
      :log-path="logPath"
    />

    <!-- View added by a plugin -->
    <PluginSlotItem
      v-else-if="activePluginView"
      :key="activePluginView.slot"
      :registration="activePluginView.registration"
      :context="pluginViewContext"
    />

    <FooterToolBar v-if="logPath">
      <AButton @click="router.go(-1)">
        {{ $gettext('Back') }}
      </AButton>
    </FooterToolBar>
  </ACard>
</template>
