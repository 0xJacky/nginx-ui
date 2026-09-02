<script setup lang="ts">
import type { NgxConfig } from '@/api/ngx'
import { AreaChartOutlined, FileExclamationOutlined, FileTextOutlined } from '@antdv-next/icons'
import { message } from 'antdv-next'
import nginxLog from '@/api/nginx_log'
import { useNgxConfigStore } from './store'

const props = withDefaults(defineProps<{
  ngxConfig: NgxConfig
  curServerIdx: number
  name?: string
  context?: 'http' | 'stream'
}>(), {
  context: 'http',
})

// Cache the indexing status at module level so multiple LogEntry instances
// mounted on the same page share a single request
let indexingStatusPromise: Promise<boolean> | null = null
function getIndexingStatus(): Promise<boolean> {
  indexingStatusPromise ??= nginxLog.getAdvancedIndexingStatus()
    .then(res => !!res.enabled)
    .catch(() => false)
  return indexingStatusPromise
}

// Same for the log directory, which only changes when nginx itself is
// reconfigured
let defaultLogDirPromise: Promise<string> | null = null
function getDefaultLogDir(): Promise<string> {
  defaultLogDirPromise ??= nginxLog.getDefaultLogDir()
    .then(res => res.access_log_dir ?? '')
    .catch(() => '')
  return defaultLogDirPromise
}

const isIndexingEnabled = ref(false)

onMounted(async () => {
  isIndexingEnabled.value = await getIndexingStatus()
})

const ngxConfigStore = useNgxConfigStore()

const directives = computed(() => props.ngxConfig?.servers?.[props.curServerIdx]?.directives ?? [])

// A log directive's target is its first parameter; the rest configure the
// format and buffering.
function logTarget(params?: string): string {
  return params?.trim().split(/\s+/)[0] ?? ''
}

// "off" disables logging rather than naming a file, so it counts as no log.
function findLogPath(directive: string): string | undefined {
  return directives.value
    .filter(v => v.directive === directive)
    .map(v => logTarget(v.params))
    .find(path => path !== '' && path !== 'off')
}

const accessLogPath = computed(() => findLogPath('access_log'))
const errorLogPath = computed(() => findLogPath('error_log'))

const hasAccessLog = computed(() => !!accessLogPath.value)
const hasErrorLog = computed(() => !!errorLogPath.value)

// Name the log file after the site, falling back to its first server_name when
// the config has not been saved under a name yet.
const logFileBaseName = computed(() => {
  const fromConfigName = props.name?.replace(/\.conf$/, '')
  const fromServerName = logTarget(directives.value.find(v => v.directive === 'server_name')?.params)

  const candidate = fromConfigName || fromServerName || ''

  return candidate.replace(/[^\w.-]/g, '_').replace(/^\.+/, '') || 'site'
})

const togglingAccessLog = ref(false)

async function toggleAccessLog(checked: boolean) {
  const server = props.ngxConfig?.servers?.[props.curServerIdx]
  if (!server)
    return

  // Drop any existing directive first: turning the log on while an
  // "access_log off;" is still in place would leave two conflicting entries.
  const rest = (server.directives ?? []).filter(v => v.directive !== 'access_log')

  if (!checked) {
    ngxConfigStore.curServerDirectives = rest
    return
  }

  togglingAccessLog.value = true
  try {
    const dir = await getDefaultLogDir()
    if (!dir) {
      message.error($gettext('Could not determine the nginx log directory'))
      return
    }

    ngxConfigStore.curServerDirectives = [
      ...rest,
      {
        directive: 'access_log',
        params: `${dir.replace(/\/+$/, '')}/${logFileBaseName.value}.access.log`,
      },
    ]
  }
  finally {
    togglingAccessLog.value = false
  }
}

const router = useRouter()

function onClickAccessLog() {
  router.push({
    path: '/nginx_log/site',
    query: {
      path: accessLogPath.value,
    },
  })
}

function onClickErrorLog() {
  router.push({
    path: '/nginx_log/site',
    query: {
      path: errorLogPath.value,
    },
  })
}

function onClickAnalytics() {
  router.push({
    path: '/nginx_log/site',
    query: {
      path: accessLogPath.value,
      view: 'dashboard',
    },
  })
}

// The log viewer and indexer parse http access log formats, so the toggle is
// offered for http servers only
const showAccessLogSwitch = computed(() => props.context === 'http')

// Without any control to render, the row would still occupy its bottom margin
const hasContent = computed(() =>
  showAccessLogSwitch.value || hasAccessLog.value || hasErrorLog.value)
</script>

<template>
  <ASpace v-if="hasContent" wrap>
    <ATooltip
      v-if="showAccessLogSwitch"
      :title="hasAccessLog
        ? $gettext('Remove the access_log directive from this server')
        : $gettext('Write an access_log directive for this server, so its requests can be viewed and analysed separately')"
    >
      <label class="access-log-toggle">
        <ASwitch
          size="small"
          :checked="hasAccessLog"
          :loading="togglingAccessLog"
          @change="checked => toggleAccessLog(!!checked)"
        />
        <span>{{ $gettext('Access Log') }}</span>
      </label>
    </ATooltip>

    <AButton
      v-if="hasAccessLog"
      type="link"
      size="small"
      @click="onClickAccessLog"
    >
      <FileTextOutlined />
      {{ $gettext('Access Logs') }}
    </AButton>
    <AButton
      v-if="hasErrorLog"
      type="link"
      size="small"
      @click="onClickErrorLog"
    >
      <FileExclamationOutlined />
      {{ $gettext('Error Logs') }}
    </AButton>
    <AButton
      v-if="hasAccessLog && isIndexingEnabled"
      type="link"
      size="small"
      @click="onClickAnalytics"
    >
      <AreaChartOutlined />
      {{ $gettext('Traffic Analytics') }}
    </AButton>
  </ASpace>
</template>

<style lang="less" scoped>
// Keep the label on the same baseline as the switch and let it toggle too
.access-log-toggle {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  user-select: none;
}
</style>
