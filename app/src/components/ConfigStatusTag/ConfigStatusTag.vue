<script setup lang="ts">
import type { ConfigStatus } from '@/constants'
import { configStatusColors, configStatusLabel } from './configStatus'

defineProps<{
  status: ConfigStatus
}>()
</script>

<template>
  <ATag :color="configStatusColors[status]" variant="solid" class="m-0">
    {{ configStatusLabel(status) }}
  </ATag>
</template>
