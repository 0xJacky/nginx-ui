<script setup lang="ts">
import type { Dns01PluginState } from '@/composables/useDns01Plugin'
import { useDns01Plugin } from '@/composables/useDns01Plugin'
import InstallConfirmModal from '@/views/system/plugins/marketplace/InstallConfirmModal.vue'

const props = withDefaults(defineProps<{
  /**
   * - `challenge`: the challenge method of a new certificate.
   * - `renewal`: an existing certificate that renews through DNS-01.
   * - `providers`: the provider list of a DNS credential.
   * - `https`: DNS validation in the site HTTPS card.
   */
  variant: 'challenge' | 'renewal' | 'providers' | 'https'
  /** Providers available without the plugin, for the `providers` variant. */
  providerCount?: number
}>(), {
  providerCount: 0,
})

const dns01 = useDns01Plugin()
const { state, enabling, installOpen, pluginId } = dns01

const visible = computed(() => state.value !== 'unknown' && state.value !== 'available')

function stateLabel(value: Dns01PluginState) {
  switch (value) {
    case 'missing':
      return $gettext('not installed')
    case 'disabled':
      return $gettext('turned off')
    default:
      return $gettext('not working right now')
  }
}

const title = computed(() => {
  const current = state.value
  switch (props.variant) {
    case 'challenge':
      if (current === 'missing')
        return $gettext('DNS-01 validation is provided by the DNS-01 plugin, which is not installed yet.')
      if (current === 'disabled')
        return $gettext('The DNS-01 plugin is installed but turned off.')
      return $gettext('The DNS-01 plugin is turned on but not working right now.')
    case 'renewal':
      return $gettext('This certificate renews through the DNS-01 plugin, which is %{state}. It cannot renew automatically until the plugin is back.', { state: stateLabel(current) })
    case 'providers':
      if (!props.providerCount)
        return $gettext('DNS providers come with the DNS-01 plugin, which is %{state}.', { state: stateLabel(current) })
      if (current === 'missing')
        return $gettext('Only %{count} providers are available now. More providers come with the DNS-01 plugin, which is not installed.', { count: String(props.providerCount) })
      if (current === 'disabled')
        return $gettext('Only %{count} providers are available now. More providers come with the DNS-01 plugin, which is turned off.', { count: String(props.providerCount) })
      return $gettext('Only %{count} providers are available now. More providers come with the DNS-01 plugin, which is not working right now.', { count: String(props.providerCount) })
    default:
      return $gettext('DNS validation needs the DNS-01 plugin, which is %{state}.', { state: stateLabel(current) })
  }
})

const installLabel = computed(() => props.variant === 'challenge'
  ? $gettext('Install DNS-01 plugin')
  : $gettext('Install plugin'))

// The challenge hint always offers the plugins page; elsewhere only when no
// button can fix it from here.
const showPluginsLink = computed(() => props.variant === 'challenge' || state.value === 'broken')
</script>

<template>
  <div v-if="visible">
    <AAlert
      :type="variant === 'https' ? 'info' : 'warning'"
      show-icon
      :title="title"
    >
      <template #description>
        <AFlex wrap gap="small" align="center">
          <AButton
            v-if="state === 'missing'"
            type="primary"
            size="small"
            @click="dns01.install"
          >
            {{ installLabel }}
          </AButton>
          <AButton
            v-else-if="state === 'disabled'"
            type="primary"
            size="small"
            :loading="enabling"
            @click="dns01.enable"
          >
            {{ $gettext('Enable plugin') }}
          </AButton>
          <AButton
            v-if="showPluginsLink"
            type="link"
            size="small"
            class="px-0"
            @click="dns01.goToPlugins"
          >
            {{ $gettext('Go to System > Plugins') }}
          </AButton>
        </AFlex>
      </template>
    </AAlert>
    <InstallConfirmModal
      v-model:open="installOpen"
      :plugin-id="pluginId"
      @installed="dns01.onInstalled"
    />
  </div>
</template>
