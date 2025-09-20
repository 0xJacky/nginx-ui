<script setup lang="ts">
import type ReconnectingWebSocket from 'reconnecting-websocket'
import type { NginxLogData } from '@/api/nginx_log'
import { useElementSize } from '@vueuse/core'
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
// Line-based storage for virtualization
const lines = ref<string[]>([])
const tailFragment = ref('') // carry over partial line when appending
const page = ref(0)
const loading = ref(false)
const filter = ref('')
// Whether to follow the log tail (auto-scroll only when near bottom)
const isFollowingBottom = ref(true)

// Setup log control data
const control = computed<NginxLogData>(() => ({
  type: props.logType,
  path: props.logPath,
}))

// Filtering
const filterRegex = computed<RegExp | null>(() => {
  if (!filter.value)
    return null
  try {
    return new RegExp(filter.value)
  }
  catch {
    return null
  }
})

const filteredIndices = computed<number[]>(() => {
  const total = lines.value.length
  if (!filterRegex.value)
    return Array.from({ length: total }, (_, i) => i)
  const regex = filterRegex.value
  const result: number[] = []
  for (let i = 0; i < total; i++) {
    if (regex!.test(lines.value[i]))
      result.push(i)
  }
  return result
})

// Virtual scroll measurements
const lineHeight = ref(24)
const scrollTop = ref(0)
const { height: containerHeight } = useElementSize(logContainer)
// Dynamic overscan: 3x viewport lines, min 60 lines
const overscanLines = computed(() => {
  const vh = containerHeight.value || 0
  const linesInView = Math.max(1, Math.ceil(vh / lineHeight.value))
  return Math.max(60, linesInView * 3)
})

const totalCount = computed(() => filteredIndices.value.length)
const atTop = ref(true)
const atBottom = ref(false)
const visibleStartIndex = computed(() => {
  if (atTop.value)
    return 0
  const start = Math.floor(scrollTop.value / lineHeight.value) - overscanLines.value
  // Also ensure we don't start beyond the last full window
  const viewportLines = Math.ceil((containerHeight.value || 0) / lineHeight.value)
  const maxStart = Math.max(0, totalCount.value - (viewportLines + overscanLines.value))
  return Math.min(Math.max(0, start), maxStart)
})
const visibleEndIndex = computed(() => {
  if (atBottom.value)
    return totalCount.value
  const viewportLines = Math.ceil((containerHeight.value || 0) / lineHeight.value)
  const maxVisible = viewportLines + overscanLines.value * 2
  const end = visibleStartIndex.value + Math.max(1, maxVisible)
  return Math.min(totalCount.value, end)
})
const topPaddingPx = computed(() => {
  // Clamp to avoid overshoot near top causing visible gap
  const px = visibleStartIndex.value * lineHeight.value
  return Math.max(0, Math.min(px, Math.max(0, totalCount.value * lineHeight.value - (containerHeight.value || 0))))
})
const bottomPaddingPx = computed(() => {
  // Ensure top+visible+bottom equals total height; avoid negative
  const totalHeight = totalCount.value * lineHeight.value
  const visibleHeight = (visibleEndIndex.value - visibleStartIndex.value) * lineHeight.value
  const px = totalHeight - topPaddingPx.value - visibleHeight
  return Math.max(0, px)
})
const visibleLines = computed(() => {
  const idxs = filteredIndices.value.slice(visibleStartIndex.value, visibleEndIndex.value)
  return idxs.map(i => lines.value[i])
})

// Style objects to avoid string concatenation in template
const topPaddingStyle = computed(() => ({ height: `${topPaddingPx.value}px` }))
const bottomPaddingStyle = computed(() => ({ height: `${bottomPaddingPx.value}px` }))

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

function isNearBottom(elem: HTMLElement, thresholdPx: number = 40): boolean {
  return elem.scrollTop + elem.clientHeight >= elem.scrollHeight - thresholdPx
}

function addLog(data: string, prepend: boolean = false) {
  const elem = logContainer.value as HTMLElement | undefined

  // Prepend: keep viewport stable by compensating with added lines
  if (prepend) {
    const parts = data.split('\n')
    if (parts.length && parts[parts.length - 1] === '')
      parts.pop()

    const addedCount = parts.length
    if (addedCount === 0)
      return

    const prevScrollTop = elem?.scrollTop ?? 0
    lines.value = parts.concat(lines.value)
    nextTick(() => {
      if (elem)
        elem.scrollTop = prevScrollTop + addedCount * lineHeight.value
      if (elem)
        scrollTop.value = elem.scrollTop
    })
    return
  }

  // Append: only auto-scroll to bottom when user is near bottom or following
  const shouldAutoScroll = elem ? (isFollowingBottom.value || isNearBottom(elem)) : true

  const chunk = tailFragment.value + data
  const parts = chunk.split('\n')
  tailFragment.value = parts.pop() ?? ''
  if (parts.length)
    lines.value.push(...parts)

  nextTick(() => {
    if (shouldAutoScroll && logContainer.value) {
      logContainer.value.scroll({
        top: logContainer.value.scrollHeight,
        left: 0,
      })
      scrollTop.value = logContainer.value.scrollTop
    }
  })
}

function clearLog() {
  lines.value = []
  tailFragment.value = ''
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
// Prefetch threshold: start preloading when near top long before hitting it
const prefetchTopThresholdPx = computed(() => {
  const vh = containerHeight.value || 0
  const minPx = lineHeight.value * 120 // at least ~120 lines
  return Math.max(minPx, vh * 1.25)
})

function prefetchIfNeeded() {
  if (loading.value || page.value <= 0)
    return
  const elem = logContainer.value as HTMLElement | undefined
  if (!elem)
    return
  // Early prefetch when top padding is small (close to top)
  if (topPaddingPx.value <= prefetchTopThresholdPx.value) {
    loading.value = true
    nginx_log.page(page.value, control.value).then(r => {
      page.value = r.page - 1
      addLog(r.content, true)
    }).finally(() => {
      loading.value = false
    })
  }
}

const debouncedPrefetch = debounce(prefetchIfNeeded, 80)

function onScroll() {
  const elem = logContainer.value as HTMLElement | undefined
  if (elem) {
    scrollTop.value = elem.scrollTop
    isFollowingBottom.value = isNearBottom(elem)
    const vh = containerHeight.value || 0
    atTop.value = elem.scrollTop <= 1
    const maxScrollTop = Math.max(0, totalCount.value * lineHeight.value - vh)
    atBottom.value = maxScrollTop - elem.scrollTop <= 1
  }
  debouncedPrefetch()
}

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
  // Try to measure line height after mount
  nextTick(() => {
    const elem = logContainer.value as HTMLElement | undefined
    if (!elem)
      return
    const probe = document.createElement('div')
    probe.className = 'nginx-log-line'
    probe.textContent = 'A'
    probe.style.visibility = 'hidden'
    probe.style.position = 'absolute'
    elem.appendChild(probe)
    const h = probe.getBoundingClientRect().height
    elem.removeChild(probe)
    if (h)
      lineHeight.value = h
  })
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

    <!-- Raw Log Display (virtualized) -->
    <ACard>
      <div
        ref="logContainer"
        class="nginx-log-container"
        @scroll="onScroll"
      >
        <div :style="topPaddingStyle" />
        <div
          v-for="(line, idx) in visibleLines"
          :key="visibleStartIndex + idx"
          class="nginx-log-line"
          v-text="line"
        />
        <div :style="bottomPaddingStyle" />
      </div>
    </ACard>
  </div>
</template>

<style lang="less">
.nginx-log-container {
  height: 60vh;
  padding: 0;
  margin: 0;

  font-size: 12px;
  line-height: 2;
  overflow: auto;
}

.nginx-log-line {
  white-space: pre; // prevent wrapping to keep constant line height
}
</style>
