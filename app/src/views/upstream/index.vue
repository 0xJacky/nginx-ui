<script setup lang="ts">
import type { EntryItem } from '@/components/EntryGrid/types'
import EntryGrid from '@/components/EntryGrid/EntryGrid.vue'

const entries = computed<EntryItem[]>(() => [
  {
    title: $gettext('Upstream Groups'),
    description: $gettext('Create shared upstream groups with load balancing and reuse them across sites.'),
    path: '/upstream/list',
  },
  {
    title: $gettext('Upstream Sockets'),
    description: $gettext('Monitor the health of every proxy target and toggle its health check.'),
    path: '/upstream/sockets',
  },
])
</script>

<template>
  <EntryGrid :entries="entries" />
</template>
