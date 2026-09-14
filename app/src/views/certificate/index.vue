<script setup lang="ts">
import EntryGrid from '@/components/EntryGrid/EntryGrid.vue'

interface EntryItem {
  title: string
  description: string
  path: string
}

const entries = computed<EntryItem[]>(() => [
  {
    title: $gettext('Certificates List'),
    description: $gettext('View all issued and imported certificates with expiration and status.'),
    path: '/certificates/list',
  },
  {
    title: $gettext('ACME User'),
    description: $gettext('Manage ACME accounts for automated certificate issuance and renewal.'),
    path: '/certificates/acme_users',
  },
  {
    title: $gettext('Import Certificate'),
    description: $gettext('Import an existing certificate and key pair into the certificate store.'),
    path: '/certificates/import',
  },
])
</script>

<template>
  <EntryGrid :entries="entries" />
</template>
