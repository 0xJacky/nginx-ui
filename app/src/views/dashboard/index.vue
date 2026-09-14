<script setup lang="ts">
import EntryGrid from '@/components/EntryGrid/EntryGrid.vue'

interface EntryItem {
  title: string
  description: string
  path: string
}

const entries = computed<EntryItem[]>(() => [
  {
    title: $gettext('Server'),
    description: $gettext('Inspect overall system status, CPU, memory, and runtime metrics.'),
    path: '/dashboard/server',
  },
  {
    title: $gettext('Nginx'),
    description: $gettext('Review Nginx performance indicators and process-level insights.'),
    path: '/dashboard/nginx',
  },
  {
    title: $gettext('Sites'),
    description: $gettext('Navigate configured sites and monitor availability and health checks.'),
    path: '/dashboard/sites',
  },
])
</script>

<template>
  <EntryGrid :entries="entries" />
</template>
