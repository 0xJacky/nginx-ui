<script setup lang="ts">
import type { EntryItem } from '@/components/EntryGrid/types'
import EntryGrid from '@/components/EntryGrid/EntryGrid.vue'

const entries = computed<EntryItem[]>(() => [
  {
    title: $gettext('Sites List'),
    description: $gettext('Browse all configured sites and manage status, logs, and operations.'),
    path: '/sites/list',
  },
  {
    title: $gettext('Add Site'),
    description: $gettext('Create a new site with quick setup or advanced configuration.'),
    path: '/sites/add',
  },
])
</script>

<template>
  <EntryGrid :entries="entries" />
</template>
