<script setup lang="ts">
import type { EntryItem } from '@/components/EntryGrid/types'
import EntryGrid from '@/components/EntryGrid/EntryGrid.vue'

const entries = computed<EntryItem[]>(() => [
  {
    title: $gettext('Access Logs'),
    description: $gettext('Read access logs page by page or follow them as they arrive.'),
    path: '/nginx_log/access',
  },
  {
    title: $gettext('Error Logs'),
    description: $gettext('Inspect error events and troubleshoot issues from Nginx error output.'),
    path: '/nginx_log/error',
  },
  {
    title: $gettext('Log List'),
    description: $gettext('See every log file Nginx writes and open one to read it.'),
    path: '/nginx_log/list',
  },
])
</script>

<template>
  <EntryGrid :entries="entries" />
</template>
