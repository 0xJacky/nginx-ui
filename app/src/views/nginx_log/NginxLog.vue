<script setup lang="ts">
import { useRouteQuery } from '@vueuse/router'
import nginxLog from '@/api/nginx_log'
import FooterToolBar from '@/components/FooterToolbar'
import PluginSlotItem from '@/components/PluginSlot/PluginSlotItem.vue'
import { NGINX_LOG_VIEW_SLOT_PREFIX, uniqueByKey } from '@/plugin/slots'
import { usePluginStore } from '@/plugin/store'
import LegacyIndexingNotice from './LegacyIndexingNotice.vue'
import RawLogViewer from './raw/RawLogViewer.vue'

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

// The raw view of the host or the key of a view a plugin registered. Empty
// means the default view of the log type.
const viewMode = useRouteQuery<string>('view', '')

const RAW_VIEW = 'raw'

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

onMounted(fetchLogPathOptions)

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

// Views plugins add next to the raw view. A key that equals the raw view is
// ignored.
const pluginStore = usePluginStore()

const pluginViewContext = computed(() => ({
  path: logPath.value,
  type: isErrorLog.value ? 'error' : 'access',
}))

const pluginViews = computed(() => uniqueByKey(pluginStore.slotsByPrefix(NGINX_LOG_VIEW_SLOT_PREFIX, pluginViewContext.value))
  .filter(item => item.key !== RAW_VIEW))

// Without a `view` in the link, access logs open the structured view a plugin
// may provide and error logs stay raw.
const requestedView = computed(() => viewMode.value || (isErrorLog.value ? RAW_VIEW : 'structured'))

const activePluginView = computed(() => pluginViews.value.find(item => item.key === requestedView.value))

// What the page shows for the requested view. A view nobody provides shows the
// raw view once plugins finished loading, until then nothing is decided yet.
const effectiveView = computed(() => {
  if (requestedView.value === RAW_VIEW || activePluginView.value)
    return requestedView.value

  return pluginStore.ready ? RAW_VIEW : ''
})

const viewModeOptions = computed(() => [
  { label: $gettext('Raw'), value: RAW_VIEW },
  ...pluginViews.value.map(item => ({
    label: $gettext(item.registration.label ?? item.key),
    value: item.key,
  })),
])

const segmentedValue = computed({
  get: () => effectiveView.value || requestedView.value,
  set: (value: string) => {
    viewMode.value = value
  },
})

// Without a plugin view there is nothing to switch to
const showViewToggle = computed(() => pluginViews.value.length > 0)
</script>

<template>
  <ACard
    :title="$gettext('Nginx Log')"
    variant="borderless"
  >
    <LegacyIndexingNotice />

    <div class="mb-4 flex flex-wrap items-center justify-end gap-4">
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

      <!-- View mode toggle, only there when a plugin adds a view -->
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

    <!-- View added by a plugin -->
    <PluginSlotItem
      v-else-if="activePluginView && effectiveView === activePluginView.key"
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
