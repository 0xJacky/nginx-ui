<script setup lang="ts">
import type { NotificationLevel } from '@/api/external_notify'
import { notificationLevelOptions, receivedLevels } from './levels'

const levels = defineModel<NotificationLevel[] | null>('value')

// Show what the notifier receives without writing a default into the form:
// an untouched selection stays empty, which the server reads as every level.
const selected = computed({
  get: () => receivedLevels(levels.value),
  set: value => {
    levels.value = value
  },
})
</script>

<template>
  <AFlex vertical :gap="4">
    <ACheckboxGroup v-model:value="selected" :options="notificationLevelOptions()" />
    <ATypographyText v-if="selected.length === 0" type="warning">
      {{ $gettext('No level is selected, so this notifier will not receive any notifications.') }}
    </ATypographyText>
  </AFlex>
</template>
