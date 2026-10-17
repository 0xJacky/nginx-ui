<script setup lang="ts">
import type { SlotContext, SlotName } from '@/plugin/types'
import { usePluginStore } from '@/plugin/store'
import PluginSlotItem from './PluginSlotItem.vue'

const props = withDefaults(defineProps<{
  /** Slot identifier, for example `certificate.challenge.form:dns01`. */
  name: SlotName
  /** Payload passed to every component mounted in the slot. */
  context?: SlotContext
}>(), {
  context: () => ({}),
})

const store = usePluginStore()

const registrations = computed(() => store.slotComponents(props.name, props.context))
</script>

<template>
  <template v-if="registrations.length > 0">
    <PluginSlotItem
      v-for="(registration, index) in registrations"
      :key="`${registration.pluginId}-${index}`"
      :registration="registration"
      :context="props.context"
    />
  </template>
  <slot v-else />
</template>
