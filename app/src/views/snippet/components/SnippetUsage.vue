<script setup lang="ts">
import ConfigUsage from '@/components/ConfigUsage'
import { toUsages } from '../usage'

const props = withDefaults(defineProps<{
  usedBy: string[]
  // Tags shown before the rest folds into the list behind "+N".
  limit?: number
}>(), { limit: 3 })

const usages = computed(() => toUsages(props.usedBy))
</script>

<template>
  <ConfigUsage
    :usages
    :limit
    :summary="$ngettext('Included by %{count} file', 'Included by %{count} files', usages.length, { count: String(usages.length) })"
  />
</template>
