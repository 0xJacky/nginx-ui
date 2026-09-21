<script setup lang="ts">
import type { SlotContext, SlotRegistration } from '@/plugin/types'
import { getErrorMessage } from '@/lib/http'

const props = defineProps<{
  registration: SlotRegistration
  context: SlotContext
}>()

const failure = ref('')

// Contain the damage: a throwing plugin component becomes an inline alert and
// the host page around it keeps rendering. The error is not rethrown.
onErrorCaptured(error => {
  failure.value = getErrorMessage(error, $gettext('Unknown error'))
  console.error(`[plugin] ${props.registration.pluginId}: slot component failed`, error)
  return false
})
</script>

<template>
  <AAlert
    v-if="failure"
    type="error"
    show-icon
    class="mb-4"
    :title="$gettext('Plugin %{plugin} failed to render', { plugin: props.registration.pluginId })"
    :description="failure"
  />
  <Suspense v-else>
    <component
      :is="props.registration.component"
      v-bind="props.context"
      :context="props.context"
    />
    <template #fallback>
      <ASkeleton active :paragraph="{ rows: 2 }" />
    </template>
  </Suspense>
</template>
