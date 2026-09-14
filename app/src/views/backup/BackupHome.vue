<script setup lang="ts">
import EntryGrid from '@/components/EntryGrid/EntryGrid.vue'

interface EntryItem {
  title: string
  description: string
  path: string
}

const entries = computed<EntryItem[]>(() => [
  {
    title: $gettext('Backup'),
    description: $gettext('Create system backups and restore from saved snapshots when needed.'),
    path: '/backup/backup-and-restore',
  },
  {
    title: $gettext('Auto Backup'),
    description: $gettext('Configure scheduled backup tasks and retention strategy automatically.'),
    path: '/backup/auto-backup',
  },
])
</script>

<template>
  <EntryGrid :entries="entries" />
</template>
