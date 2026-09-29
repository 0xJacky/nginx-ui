<script setup lang="ts">
import type { SiteLog } from '@/api/site'
import site from '@/api/site'
import PluginSlot from '@/components/PluginSlot'
import { usePluginStore } from '@/plugin/store'

const props = defineProps<{
  siteName: string
}>()

const pluginStore = usePluginStore()

// The log paths are only fetched when a plugin renders something here, so a
// host without such a plugin makes no extra request per row.
const hasPlugins = computed(() => (pluginStore.slots['site.log.actions']?.length ?? 0) > 0)

const logs = ref<SiteLog[]>([])
let fetched = false

async function fetchLogs() {
  if (fetched || !hasPlugins.value)
    return

  fetched = true
  try {
    const res = await site.getLogs(props.siteName)
    logs.value = res.logs ?? []
  }
  catch (error) {
    console.error('Failed to load the site logs:', error)
  }
}

watch(hasPlugins, fetchLogs, { immediate: true })

// The log directives of the site's own configuration, empty when it has none.
function ownLogPath(type: SiteLog['type']) {
  return logs.value.find(log => log.type === type && log.valid && !log.inherited)?.path ?? ''
}

const context = computed(() => ({
  accessLogPath: ownLogPath('access'),
  errorLogPath: ownLogPath('error'),
  siteName: props.siteName,
}))
</script>

<template>
  <PluginSlot v-if="hasPlugins" name="site.log.actions" :context="context" />
</template>
