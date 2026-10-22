<script setup lang="ts">
import type { UpstreamDiscovery } from '@/api/upstream_discovery'
import GeneratedInclude from '@/components/GeneratedInclude'
import PluginConfigForm from '@/components/PluginConfigForm'
import {
  defaultRefreshSeconds,
  discoveryProviders,
  findDiscoveryProvider,
  loadDiscoveryProviders,
  minRefreshSeconds,
  sanitizeDiscoveryConfig,
} from '../discovery'

// The provider, form values, service, interval and extra directives of an
// upstream bound to a discovered service.
const binding = defineModel<UpstreamDiscovery>({ required: true })

onMounted(() => {
  loadDiscoveryProviders(true)
  if (!binding.value.config)
    binding.value.config = {}
  if (!binding.value.id && !binding.value.refresh_seconds)
    binding.value.refresh_seconds = defaultRefreshSeconds
})

const provider = computed(() => findDiscoveryProvider(binding.value.kind))

// A binding whose plugin is gone keeps its stored provider selectable, so it
// is not changed by accident.
const providerOptions = computed(() => {
  const options = discoveryProviders.value.map(item => ({ label: item.name, value: item.kind }))
  if (binding.value.kind && !options.some(option => option.value === binding.value.kind))
    options.push({ label: binding.value.kind, value: binding.value.kind })
  return options
})

const config = computed<Record<string, string>>({
  get: () => binding.value.config ?? {},
  set: value => {
    binding.value.config = value
  },
})

// Keep only the values the selected provider declares.
watch(() => binding.value.kind, value => {
  if (findDiscoveryProvider(value))
    binding.value.config = sanitizeDiscoveryConfig(value, binding.value.config)
})
</script>

<template>
  <div>
    <AAlert
      v-if="!discoveryProviders.length"
      class="mb-4"
      type="info"
      show-icon
      :message="$gettext('No enabled plugin offers service discovery. Install and enable a plugin with the upstream.discovery capability first.')"
    />
    <AFormItem
      required
      :label="$gettext('Provider')"
    >
      <ASelect
        v-model:value="binding.kind"
        :options="providerOptions"
        :placeholder="$gettext('Select a provider')"
      />
    </AFormItem>
    <AAlert
      v-if="binding.kind && !provider"
      class="mb-4"
      type="warning"
      show-icon
      :message="$gettext('The plugin that provides this provider is not enabled. The upstream is kept but no longer refreshed.')"
    />

    <PluginConfigForm
      v-if="provider"
      v-model="config"
      :fields="provider.fields"
    />

    <AFormItem
      required
      :label="$gettext('Service')"
      :extra="$gettext('The service to resolve, in the terms of the provider.')"
    >
      <AInput v-model:value="binding.service" />
    </AFormItem>

    <AFormItem
      :label="$gettext('Refresh Interval')"
      :extra="$gettext('How often the service is resolved again, in seconds. Nginx is reloaded only when the servers changed.')"
    >
      <AInputNumber
        v-model:value="binding.refresh_seconds"
        class="w-full"
        :min="minRefreshSeconds"
      />
    </AFormItem>

    <AFormItem
      :label="$gettext('Extra Directives')"
      :extra="$gettext('Appended inside the upstream block, one directive per line, for example keepalive 32;')"
    >
      <ATextarea
        v-model:value="binding.extra_directives"
        :rows="3"
        placeholder="keepalive 32;"
      />
    </AFormItem>

    <AFormItem
      v-if="binding.include"
      :label="$gettext('Include')"
      :extra="$gettext('Add this line to the http block, for example at the top of a site configuration, then proxy to the upstream by its name.')"
    >
      <GeneratedInclude
        :include="binding.include"
        :path="binding.path"
      />
    </AFormItem>
  </div>
</template>
