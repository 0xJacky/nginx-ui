<script setup lang="ts">
import type ReconnectingWebSocket from 'reconnecting-websocket'
import type { NginxLogData } from '@/api/nginx_log'
import { debounce } from 'lodash'
import nginx_log from '@/api/nginx_log'
import ws from '@/lib/websocket'

interface Props {
  logPath: string
  logType: string
  autoRefresh: boolean
}

const props = defineProps<Props>()

// Template refs
const logContainer = useTemplateRef('logContainer')

// Reactive data
let websocket: ReconnectingWebSocket | WebSocket
const buffer = ref('')
const page = ref(0)
const loading = ref(false)
const filter = ref('')

// Setup log control data
const control = computed<NginxLogData>(() => ({
  type: props.logType,
  path: props.logPath,
}))

// Computed buffer with filtering
const computedBuffer = computed(() => {
  if (filter.value)
    return buffer.value.split('\n').filter(line => line.match(filter.value)).join('\n')

  return buffer.value
})

// WebSocket functions
function openWs() {
  websocket = ws('/api/nginx_log')

  websocket.onopen = () => {
    websocket.send(JSON.stringify(control.value))
  }

  websocket.onmessage = (m: { data: string }) => {
    addLog(`${m.data}\n`)
  }
}

function addLog(data: string, prepend: boolean = false) {
  if (prepend)
    buffer.value = data + buffer.value
  else
    buffer.value += data

  nextTick(() => {
    logContainer.value?.scroll({
      top: logContainer.value.scrollHeight,
      left: 0,
    })
  })
}

function clearLog() {
  buffer.value = ''
}

// Initialize log loading
function init() {
  nginx_log.page(0, control.value).then(r => {
    page.value = r.page - 1
    addLog(r.content)
    if (props.autoRefresh) {
      openWs()
    }
  }).catch(e => {
    if (e.error)
      addLog(T(e.error))
  })
}

// Scroll handling for pagination
function onScrollLog() {
  if (!loading.value && page.value > 0) {
    loading.value = true

    const elem = logContainer.value!

    if (elem?.scrollTop / elem?.scrollHeight < 0.333) {
      nginx_log.page(page.value, control.value).then(r => {
        page.value = r.page - 1
        addLog(r.content, true)
      }).finally(() => {
        loading.value = false
      })
    }
    else {
      loading.value = false
    }
  }
}

const debounceScrollLog = debounce(onScrollLog, 100)

// Watch for auto refresh changes
watch(() => props.autoRefresh, async value => {
  if (value) {
    clearLog()
    await nextTick()
    init()
  }
  else {
    websocket?.close()
  }
})

// Watch for control changes
watch(control, () => {
  clearLog()

  nextTick(() => {
    if (websocket && websocket.readyState === WebSocket.OPEN) {
      websocket.send(JSON.stringify(control.value))
    }
    else {
      init()
    }
  })
}, { deep: true })

// Initialize on mount
onMounted(() => {
  init()
})

// Cleanup on unmount
onUnmounted(() => {
  websocket?.close()
})

// Expose functions for parent component
defineExpose({
  clearLog,
  init,
})
</script>

<template>
  <div>
    <!-- Filter -->
    <AForm layout="vertical" class="mb-4">
      <AFormItem :label="$gettext('Filter')">
        <AInput
          v-model:value="filter"
          :placeholder="$gettext('Filter log content')"
          style="max-width: 300px"
        />
      </AFormItem>
    </AForm>

    <!-- Raw Log Display -->
    <ACard>
      <pre
        ref="logContainer"
        v-dompurify-html="computedBuffer"
        class="nginx-log-container"
        @scroll="debounceScrollLog"
      />
    </ACard>
  </div>
</template>

<style lang="less">
.nginx-log-container {
  height: 60vh;
  padding: 5px;
  margin-bottom: 0;

  font-size: 12px;
  line-height: 2;
}
</style>
