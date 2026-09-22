<script setup lang="ts">
import type { RefreshStatus } from '@/api/blocklist'
import { formatDateTime } from '@/lib/helper'

// The outcome of the last refresh of a generated nginx file.
const props = defineProps<{
  status?: RefreshStatus
  message?: string
  lastRunAt?: string | null
}>()
</script>

<template>
  <ATag v-if="!props.status">
    {{ $gettext('Not run yet') }}
  </ATag>
  <ATooltip
    v-else
    :title="props.message"
  >
    <div class="flex items-center gap-1 flex-wrap">
      <ATag :color="props.status === 'ok' ? 'green' : 'red'">
        {{ props.status === 'ok' ? $gettext('Success') : $gettext('Failed') }}
      </ATag>
      <span
        v-if="props.lastRunAt"
        class="text-xs opacity-70"
      >
        {{ formatDateTime(props.lastRunAt) }}
      </span>
    </div>
  </ATooltip>
</template>
