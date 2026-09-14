<script setup lang="ts">
import EntryGrid from '@/components/EntryGrid/EntryGrid.vue'

interface EntryItem {
  title: string
  description: string
  path: string
}

const entries = computed<EntryItem[]>(() => [
  {
    title: $gettext('DNS Domains'),
    description: $gettext('Manage DNS zones and records for domains under your account.'),
    path: '/dns/domains',
  },
  {
    title: $gettext('Credentials'),
    description: $gettext('Configure provider API credentials used for DNS automation tasks.'),
    path: '/dns/credentials',
  },
  {
    title: $gettext('Groups'),
    description: $gettext('Organize related records with reusable DNS groups for deployment.'),
    path: '/dns/groups',
  },
  {
    title: $gettext('DDNS'),
    description: $gettext('Set up dynamic DNS updates to keep records aligned with IP changes.'),
    path: '/dns/ddns',
  },
])
</script>

<template>
  <EntryGrid :entries="entries" />
</template>
