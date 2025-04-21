<script setup lang="ts">
import type { INginxLogData } from '@/api/nginx_log'
import type ReconnectingWebSocket from 'reconnecting-websocket'
import nginx_log from '@/api/nginx_log'
import FooterToolBar from '@/components/FooterToolbar'
import ws from '@/lib/websocket'
import { debounce } from 'lodash'

const logContainer = useTemplateRef('logContainer')
let websocket: ReconnectingWebSocket | WebSocket
const route = useRoute()
const buffer = ref('')
const page = ref(0)
const autoRefresh = ref(true)
const router = useRouter()
const loading = ref(false)
const filter = ref('')

// Setup log control data based on route params
const control = reactive<INginxLogData>({
  type: logType(),
  log_path: route.query.log_path as string,
})

function logType() {
  if (route.path.indexOf('access') > 0)
    return 'access'
  return route.path.indexOf('error') > 0 ? 'error' : 'site'
}

function openWs() {
  websocket = ws('/api/nginx_log')

  websocket.onopen = () => {
    websocket.send(JSON.stringify(control))
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

function init() {
  nginx_log.page(0, control).then(r => {
    page.value = r.page - 1
    addLog(r.content)
    openWs()
  }).catch(e => {
    addLog(e.error)
  })
}

function clearLog() {
  buffer.value = ''
}

onMounted(() => {
  init()
})

onUnmounted(() => {
  websocket?.close()
})

watch(autoRefresh, async value => {
  if (value) {
    clearLog()
    await nextTick()
    await init()
    openWs()
  }
  else {
    websocket.close()
  }
})

watch(route, () => {
  // Update control data when route changes
  control.type = logType()
  control.log_path = route.query.log_path as string

  clearLog()
  init()
})

watch(control, () => {
  clearLog()
  autoRefresh.value = true

  nextTick(() => {
    websocket.send(JSON.stringify(control))
  })
})

function onScrollLog() {
  if (!loading.value && page.value > 0) {
    loading.value = true

    const elem = logContainer.value!

    if (elem?.scrollTop / elem?.scrollHeight < 0.333) {
      nginx_log.page(page.value, control).then(r => {
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

function debounceScrollLog() {
  return debounce(onScrollLog, 100)()
}

const computedBuffer = computed(() => {
  if (filter.value)
    return buffer.value.split('\n').filter(line => line.match(filter.value)).join('\n')

  return buffer.value
})
</script>

<template>
  <ACard
    :title="$gettext('Nginx Log')"
    :bordered="false"
  >
    <template #extra>
      <div class="flex items-center">
        <span class="mr-2">
          {{ $gettext('Auto Refresh') }}
        </span>
        <ASwitch v-model:checked="autoRefresh" />
      </div>
    </template>
    <AForm layout="vertical">
      <AFormItem :label="$gettext('Filter')">
        <AInput
          v-model:value="filter"
          style="max-width: 300px"
        />
      </AFormItem>
    </AForm>

    <ACard>
      <pre
        ref="logContainer"
        v-dompurify-html="computedBuffer"
        class="nginx-log-container"
        @scroll="debounceScrollLog"
      />
    </ACard>
    <FooterToolBar v-if="control.log_path">
      <AButton @click="router.go(-1)">
        {{ $gettext('Back') }}
      </AButton>
    </FooterToolBar>
  </ACard>
</template>

<style lang="less">
.nginx-log-container {
  height: 60vh;
  overflow: scroll;
  padding: 5px;
  margin-bottom: 0;

  font-size: 12px;
  line-height: 2;
}
</style>
