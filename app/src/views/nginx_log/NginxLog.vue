<script setup lang="ts">
import type { INginxLogData } from '@/api/nginx_log'
import type ReconnectingWebSocket from 'reconnecting-websocket'
import nginx_log from '@/api/nginx_log'
import FooterToolBar from '@/components/FooterToolbar/FooterToolBar.vue'
import ws from '@/lib/websocket'
import { debounce } from 'lodash'

const logContainer = useTemplateRef('logContainer')
let websocket: ReconnectingWebSocket | WebSocket
const route = useRoute()
const buffer = ref('')
const page = ref(0)
const auto_refresh = ref(true)
const router = useRouter()
const loading = ref(false)
const filter = ref('')

const control = reactive<INginxLogData>({
  type: logType(),
  conf_name: route.query.conf_name as string,
  server_idx: Number.parseInt(route.query.server_idx as string),
  directive_idx: Number.parseInt(route.query.directive_idx as string),
})

function logType() {
  if (route.path.indexOf('access') > 0)
    return 'access'
  return route.path.indexOf('error') > 0 ? 'error' : 'site'
}

function openWs() {
  websocket = ws('/api/nginx_log')

  websocket.onopen = () => {
    websocket.send(JSON.stringify({
      ...control,
    }))
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
  logContainer.value!.innerHTML = ''
}

onMounted(() => {
  init()
})

onUnmounted(() => {
  websocket?.close()
})

watch(auto_refresh, value => {
  if (value) {
    openWs()
    clearLog()
  }
  else {
    websocket.close()
  }
})

watch(route, () => {
  init()

  control.type = logType()
  control.directive_idx = Number.parseInt(route.query.server_idx as string)
  control.server_idx = Number.parseInt(route.query.directive_idx as string)
  clearLog()

  nextTick(() => {
    websocket.send(JSON.stringify(control))
  })
})

watch(control, () => {
  clearLog()
  auto_refresh.value = true

  nextTick(() => {
    websocket.send(JSON.stringify(control))
  })
})

function on_scroll_log() {
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

function debounce_scroll_log() {
  return debounce(on_scroll_log, 100)()
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
    <AForm layout="vertical">
      <AFormItem :label="$gettext('Auto Refresh')">
        <ASwitch v-model:checked="auto_refresh" />
      </AFormItem>
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
        @scroll="debounce_scroll_log"
      />
    </ACard>
    <FooterToolBar v-if="control.type === 'site'">
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
