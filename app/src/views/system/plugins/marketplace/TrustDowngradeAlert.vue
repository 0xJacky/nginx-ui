<script setup lang="ts">
import type { PluginTrust } from '@/api/plugin_marketplace'
import { isTrustDowngrade, packageTrustPreset } from './trust'

const props = defineProps<{
  /** Trust of the package about to be installed. */
  next?: PluginTrust
  /** Trust of the version already installed. */
  installed?: PluginTrust
}>()

const isDowngrade = computed(() => isTrustDowngrade(props.next, props.installed))
const nextLabel = computed(() => packageTrustPreset(props.next)?.label() ?? '')
const installedLabel = computed(() => packageTrustPreset(props.installed)?.label() ?? '')
</script>

<template>
  <AAlert
    v-if="isDowngrade"
    type="warning"
    show-icon
    :title="$gettext('This package is trusted less than the version already installed')"
    :description="$gettext('Installed version: %{installed}. This package: %{package}.', {
      installed: installedLabel,
      package: nextLabel,
    })"
  />
</template>
