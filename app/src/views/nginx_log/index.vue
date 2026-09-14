<script setup lang="ts">
import EntryGrid from '@/components/EntryGrid/EntryGrid.vue'

interface EntryItem {
  title: string
  description: string
  path: string
}

const entries = computed<EntryItem[]>(() => [
  {
    title: $gettext('Access Logs'),
    description: $gettext('Explore access traffic logs with filters and detailed request information.'),
    path: '/nginx_log/access',
  },
  {
    title: $gettext('Error Logs'),
    description: $gettext('Inspect error events and troubleshoot issues from Nginx error output.'),
    path: '/nginx_log/error',
  },
  {
    title: $gettext('Log List'),
    description: $gettext('Manage indexed log files and jump into structured analysis workflows.'),
    path: '/nginx_log/list',
  },
])
</script>

<template>
  <EntryGrid :entries="entries" />
</template>
