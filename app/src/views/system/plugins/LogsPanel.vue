<script setup lang="ts">
import type { PluginInfo, PluginLogLine } from '@/api/plugin'
import { CopyOutlined, ReloadOutlined } from '@antdv-next/icons'
import { useClipboard, useIntervalFn } from '@vueuse/core'
import pluginApi from '@/api/plugin'
import { getErrorMessage } from '@/lib/http'

const props = defineProps<{
  plugin?: PluginInfo
  /** Lines are fetched and tailed only while the panel is visible. */
  active: boolean
}>()

const { message } = useGlobalApp()
const { copy, isSupported: isClipboardSupported } = useClipboard()

const loading = ref(false)
const error = ref('')
const lines = ref<PluginLogLine[]>([])
const follow = ref(true)
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
    if (follow.value && viewport.value)
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

watch(() => [props.active, props.plugin?.id] as const, ([active, id]) => {
  if (active && id) {
    lines.value = []
    void load(true)
    resume()
  }
  else {
    pause()
  }
}, { immediate: true })

onUnmounted(pause)
</script>

<template>
  <div class="logs-panel">
    <div class="mb-3 flex flex-wrap items-center justify-between gap-2">
      <div class="flex items-center gap-2 text-sm">
        <ASwitch v-model:checked="follow" size="small" />
        <span>{{ $gettext('Follow new lines') }}</span>
      </div>
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
    </div>

    <AAlert
      v-if="error"
      type="error"
      show-icon
      class="mb-3"
      :title="error"
    />

    <ASpin :spinning="loading">
      <div ref="viewport" class="log-viewport">
        <pre v-if="text" class="log-content">{{ text }}</pre>
        <AEmpty v-else :description="$gettext('No log output yet')" />
      </div>
    </ASpin>
  </div>
</template>

<style lang="less" scoped>
.log-viewport {
  height: calc(100vh - 280px);
  min-height: 320px;
  overflow: auto;
  padding: 12px;
  border-radius: var(--ant-border-radius);
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
