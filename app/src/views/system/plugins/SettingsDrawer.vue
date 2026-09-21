<script setup lang="ts">
import type { PluginInfo, SettingsSchema } from '@/api/plugin'
import { useWindowSize } from '@vueuse/core'
import pluginApi from '@/api/plugin'
import PluginSlot from '@/components/PluginSlot'
import { getErrorMessage } from '@/lib/http'
import { usePluginStore } from '@/plugin'
import SchemaForm from './SchemaForm.vue'

const props = defineProps<{
  plugin?: PluginInfo
}>()

const open = defineModel<boolean>('open', { default: false })

const { message } = App.useApp()
const pluginStore = usePluginStore()

const { width: windowWidth } = useWindowSize()
// Take the whole screen on a phone instead of leaving an unusable sliver.
const drawerSize = computed(() => Math.min(520, windowWidth.value))

const loading = ref(false)
const saving = ref(false)
const error = ref('')
const schema = ref<SettingsSchema | null>(null)
const values = ref<Record<string, unknown>>({})

/** A plugin may replace the generated form with its own component. */
const customPanel = computed(() => {
  const id = props.plugin?.id
  return id ? pluginStore.settingsPanels[id] : undefined
})

async function load() {
  const id = props.plugin?.id
  if (!id)
    return

  loading.value = true
  error.value = ''
  try {
    const data = await pluginApi.getSettings(id)
    schema.value = data.schema ?? props.plugin?.settings_schema ?? null
    values.value = { ...(data.values ?? {}) }
  }
  catch (e) {
    error.value = getErrorMessage(e, $gettext('Failed to load the plugin settings'))
  }
  finally {
    loading.value = false
  }
}

/**
 * Secret fields come back as a placeholder. Sending it back unchanged is what
 * tells the backend to keep the value it already stores.
 */
async function save(next?: Record<string, unknown>) {
  const id = props.plugin?.id
  if (!id)
    return

  saving.value = true
  error.value = ''
  try {
    const data = await pluginApi.saveSettings(id, next ?? values.value)
    schema.value = data.schema ?? schema.value
    values.value = { ...(data.values ?? {}) }
    message.success($gettext('Plugin settings saved'))
  }
  catch (e) {
    error.value = getErrorMessage(e, $gettext('Failed to save the plugin settings'))
  }
  finally {
    saving.value = false
  }
}

watch(open, value => {
  if (value)
    load()
})
</script>

<template>
  <ADrawer
    v-model:open="open"
    :title="$gettext('Settings: %{name}', { name: props.plugin?.name ?? '' })"
    :size="drawerSize"
    placement="right"
  >
    <ASpin :spinning="loading">
      <AAlert
        v-if="error"
        type="error"
        show-icon
        class="mb-4"
        :title="error"
      />

      <component
        :is="customPanel"
        v-if="customPanel"
        :settings="values"
        :save="save"
      />
      <SchemaForm
        v-else-if="schema?.settings?.length"
        v-model:values="values"
        :schema="schema"
        :saving="saving"
        @save="save()"
      />
      <AEmpty v-else-if="!loading" :description="$gettext('This plugin has no settings.')" />

      <PluginSlot
        v-if="props.plugin"
        :name="`plugin.settings:${props.plugin.id}`"
        :context="{ settings: values }"
      />
    </ASpin>
  </ADrawer>
</template>
