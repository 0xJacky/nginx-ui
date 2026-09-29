<script setup lang="ts">
import type { Site } from '@/api/site'
import PluginSlot from '@/components/PluginSlot'
import { siteLogContext } from '@/plugin/siteLogContext'
import { usePluginStore } from '@/plugin/store'

const props = defineProps<{
  site: Pick<Site, 'name' | 'access_log_path' | 'access_log_inherited' | 'error_log_path' | 'error_log_inherited'>
}>()

const pluginStore = usePluginStore()

// The list row already carries the log paths, so nothing is requested per row.
const context = computed(() => siteLogContext({
  siteName: props.site.name,
  accessLogPath: props.site.access_log_path,
  accessLogInherited: props.site.access_log_inherited,
  errorLogPath: props.site.error_log_path,
  errorLogInherited: props.site.error_log_inherited,
}))

const hasPlugins = computed(() => pluginStore.slotComponents('site.log.actions', context.value).length > 0)
</script>

<template>
  <PluginSlot v-if="hasPlugins" name="site.log.actions" :context="context" />
</template>
