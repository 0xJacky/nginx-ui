<script setup lang="ts">
import type { BlocklistSource } from '@/api/blocklist'
import GeneratedInclude from '@/components/GeneratedInclude'
import PluginConfigForm from '@/components/PluginConfigForm'
import {
  blocklistKinds,
  findBlocklistKind,
  loadBlocklistKinds,
  minRefreshSeconds,
  sanitizeBlocklistConfig,
} from '../blocklists'

// The kind, form values and refresh interval of a blocklist source.
const source = defineModel<BlocklistSource>({ required: true })

onMounted(() => {
  loadBlocklistKinds(true)
  if (!source.value.config)
    source.value.config = {}
})

const kind = computed(() => findBlocklistKind(source.value.kind))

// A source whose plugin is gone keeps its stored kind selectable, so it is
// not changed by accident.
const kindOptions = computed(() => {
  const options = blocklistKinds.value.map(item => ({ label: item.name, value: item.kind }))
  if (source.value.kind && !options.some(option => option.value === source.value.kind))
    options.push({ label: source.value.kind, value: source.value.kind })
  return options
})

const config = computed<Record<string, string>>({
  get: () => source.value.config ?? {},
  set: value => {
    source.value.config = value
  },
})

// Keep only the values the selected kind declares, and start a new source
// from the interval of its kind.
watch(() => source.value.kind, value => {
  const found = findBlocklistKind(value)
  if (!found)
    return
  source.value.config = sanitizeBlocklistConfig(value, source.value.config)
  if (!source.value.id)
    source.value.refresh_seconds = found.refresh_seconds
})
</script>

<template>
  <div>
    <AAlert
      v-if="!blocklistKinds.length"
      class="mb-4"
      type="info"
      show-icon
      :message="$gettext('No enabled plugin offers a blocklist source. Install and enable a plugin with the security.blocklist capability first.')"
    />
    <AFormItem
      required
      :label="$gettext('Source Type')"
    >
      <ASelect
        v-model:value="source.kind"
        :options="kindOptions"
        :placeholder="$gettext('Select a source type')"
      />
    </AFormItem>
    <AAlert
      v-if="source.kind && !kind"
      class="mb-4"
      type="warning"
      show-icon
      :message="$gettext('The plugin that provides this source type is not enabled. The list is kept but no longer refreshed.')"
    />

    <PluginConfigForm
      v-if="kind"
      v-model="config"
      :fields="kind.fields"
    />

    <AFormItem
      :label="$gettext('Refresh Interval')"
      :extra="$gettext('How often the list is fetched again, in seconds. Nginx is reloaded only when the list changed.')"
    >
      <AInputNumber
        v-model:value="source.refresh_seconds"
        class="w-full"
        :min="minRefreshSeconds"
        :step="60"
        :placeholder="kind ? String(kind.refresh_seconds) : ''"
      />
    </AFormItem>

    <AFormItem
      v-if="source.include"
      :label="$gettext('Include')"
      :extra="$gettext('Add this line to a server or location block to deny the listed addresses there.')"
    >
      <GeneratedInclude
        :include="source.include"
        :path="source.path"
      />
    </AFormItem>
  </div>
</template>
