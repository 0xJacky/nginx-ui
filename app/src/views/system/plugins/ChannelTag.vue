<script setup lang="ts">
import type { PluginChannel } from './channel'
import { channelHint, channelLabel } from './channel'

// The stable channel is the usual case and shows nothing.
defineProps<{
  channel: PluginChannel
}>()
</script>

<template>
  <ATooltip v-if="channel !== 'stable'" :title="channelHint(channel)">
    <ATag :color="channel === 'beta' ? 'warning' : 'purple'" class="m-0">
      {{ channelLabel(channel) }}
    </ATag>
  </ATooltip>
</template>
