<script setup lang="ts">
import type { PluginInfo, PluginLogLine } from '@/api/plugin'
import { CopyOutlined, ReloadOutlined } from '@antdv-next/icons'
import { useClipboard, useIntervalFn, useWindowSize } from '@vueuse/core'
import pluginApi from '@/api/plugin'
import { getErrorMessage } from '@/lib/http'

const props = defineProps<{
  plugin?: PluginInfo
}>()

const open = defineModel<boolean>('open', { default: false })

const { message } = App.useApp()
const { copy, isSupported: isClipboardSupported } = useClipboard()

const { width: windowWidth } = useWindowSize()
const drawerSize = computed(() => Math.min(720, windowWidth.value))

const loading = ref(false)
const error = ref('')
const lines = ref<PluginLogLine[]>([])
const viewport = useTemplateRef<HTMLElement>('viewport')

const text = computed(() => lines.value.map(item => `${item.time} ${item.line}`).join('\n'))

async function load(showSpinner = false) {
  const id = props.plugin?.id
  if (!id)
    return

  if (showSpinner)
    loading.value = true

  try {
    const data = await pluginApi.getLogs(id, 500)
    lines.value = data.lines ?? []
    error.value = ''
    await nextTick()
    // Keep the newest line in view, the way a tail would.
    if (viewport.value)
      viewport.value.scrollTop = viewport.value.scrollHeight
  }
  catch (e) {
    error.value = getErrorMessage(e, $gettext('Failed to load the plugin logs'))
  }
  finally {
    loading.value = false
  }
}

const { pause, resume } = useIntervalFn(() => load(), 5000, { immediate: false })

async function copyLogs() {
  try {
    await copy(text.value)
    message.success($gettext('Logs copied to clipboard'))
  }
  catch {
    message.error($gettext('Failed to copy the logs'))
  }
}

watch(open, value => {
  if (value) {
    lines.value = []
    load(true)
    resume()
  }
  else {
    pause()
  }
})

onUnmounted(pause)
</script>

<template>
  <ADrawer
    v-model:open="open"
    :title="$gettext('Logs: %{name}', { name: props.plugin?.name ?? '' })"
    :size="drawerSize"
    placement="right"
  >
    <template #extra>
      <ASpace>
        <AButton size="small" :loading="loading" @click="load(true)">
          <template #icon>
            <ReloadOutlined />
          </template>
          {{ $gettext('Refresh') }}
        </AButton>
        <AButton
          v-if="isClipboardSupported"
          size="small"
          :disabled="!text"
          @click="copyLogs"
        >
          <template #icon>
            <CopyOutlined />
          </template>
          {{ $gettext('Copy') }}
        </AButton>
      </ASpace>
    </template>

    <AAlert
      v-if="error"
      type="error"
      show-icon
      class="mb-4"
      :title="error"
    />

    <ASpin :spinning="loading">
      <div ref="viewport" class="log-viewport">
        <pre v-if="text" class="log-content">{{ text }}</pre>
        <AEmpty v-else :description="$gettext('No log output yet')" />
      </div>
    </ASpin>
  </ADrawer>
</template>

<style lang="less" scoped>
.log-viewport {
  height: calc(100vh - 160px);
  overflow: auto;
  padding: 12px;
  border-radius: 6px;
  background-color: var(--ant-color-fill-quaternary);
}

.log-content {
  margin: 0;
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 12px;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-all;
  color: var(--ant-color-text);
}
</style>
