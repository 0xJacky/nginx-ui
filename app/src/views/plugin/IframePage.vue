<script setup lang="ts">
import { useEventListener } from '@vueuse/core'
import { useSettingsStore, useUserStore } from '@/pinia'

const props = defineProps<{
  /** Plugin that owns the page. */
  pluginId: string
  /** File inside the plugin `pages/` directory, as declared in the manifest. */
  file: string
}>()

const settings = useSettingsStore()
const user = useUserStore()

const frame = useTemplateRef<HTMLIFrameElement>('frame')

// Each segment is encoded on its own, so a file name with a space, `#`, `?`
// or `%` still reaches the right file and `/` keeps separating directories.
const encodedFile = computed(() => props.file.split('/').map(segment => encodeURIComponent(segment)).join('/'))

// Relative on purpose: the whole app is served under the same prefix, so this
// keeps working behind a reverse proxy that mounts Nginx UI on a sub path.
const src = computed(() => `./plugins/${encodeURIComponent(props.pluginId)}/pages/${encodedFile.value}`)

const theme = computed(() => (settings.theme === 'dark' ? 'dark' : 'light'))

/**
 * Bridge for zero-build pages. A page asks for the auth token or the current
 * theme and the host answers on the same channel. Messages from anything other
 * than this iframe are ignored, and replies never leave the current origin.
 */
function handleMessage(event: MessageEvent) {
  const frameWindow = frame.value?.contentWindow
  if (!frameWindow || event.source !== frameWindow)
    return

  const type = (event.data as { type?: string } | null)?.type

  if (type === 'nginx-ui:token') {
    frameWindow.postMessage({ type: 'nginx-ui:token', token: user.token }, window.location.origin)
    return
  }

  if (type === 'nginx-ui:theme')
    frameWindow.postMessage({ type: 'nginx-ui:theme', theme: theme.value }, window.location.origin)
}

useEventListener(window, 'message', handleMessage)

// Push theme changes without waiting for the page to ask again.
watch(theme, value => {
  frame.value?.contentWindow?.postMessage({ type: 'nginx-ui:theme', theme: value }, window.location.origin)
})
</script>

<template>
  <ACard :styles="{ body: { padding: 0 } }">
    <iframe
      ref="frame"
      class="plugin-iframe"
      :src="src"
      :title="props.pluginId"
    />
  </ACard>
</template>

<style lang="less" scoped>
.plugin-iframe {
  display: block;
  width: 100%;
  height: calc(100vh - 260px);
  min-height: 420px;
  border: none;
}

@media (max-width: 512px) {
  .plugin-iframe {
    height: calc(100vh - 220px);
  }
}
</style>
