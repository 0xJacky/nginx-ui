<script setup lang="ts">
import EntryGrid from '@/components/EntryGrid/EntryGrid.vue'

interface EntryItem {
  title: string
  description: string
  path: string
}

const entries = computed<EntryItem[]>(() => [
  {
    title: $gettext('About'),
    description: $gettext('View version details and core information about this Nginx UI instance.'),
    path: '/system/about',
  },
  {
    title: $gettext('Self Check'),
    description: $gettext('Run built-in diagnostics to validate environment and service readiness.'),
    path: '/system/self_check',
  },
  {
    title: $gettext('Upgrade'),
    description: $gettext('Check release updates and execute upgrade operations when available.'),
    path: '/system/upgrade',
  },
  {
    title: $gettext('Third-party Components'),
    description: $gettext('Review third-party licenses and open source component attributions.'),
    path: '/system/licenses',
  },
])
</script>

<template>
  <EntryGrid :entries="entries" />
</template>
