<script setup lang="ts">
import type { Component } from 'vue'
import type { EntryItem } from '@/components/EntryGrid/types'
import { FileOutlined } from '@antdv-next/icons'
import EntryGrid from '@/components/EntryGrid/EntryGrid.vue'
import { configFavoriteDir, configFavoriteRoute, useConfigFavorites } from '@/composables/useConfigFavorites'

interface HomeSection {
  key: string
  label: string
  entries: EntryItem[]
}

const router = useRouter()
const { favorites } = useConfigFavorites()

/**
 * Reuse the sidebar icon of the destination so a card and its menu item are
 * recognisably the same entry point. Routes without an icon render a plain card.
 */
function resolveRouteIcon(path: string): Component | undefined {
  const { matched } = router.resolve(path)

  for (let i = matched.length - 1; i >= 0; i--) {
    const icon = matched[i].meta?.icon
    if (icon)
      return markRaw(icon)
  }

  return undefined
}

/**
 * Config files the user starred, pinned above the fixed entry points. Hidden
 * entirely while nothing is starred so the hub stays quiet by default.
 */
const favoriteSection = computed<HomeSection | undefined>(() => {
  if (favorites.value.length === 0)
    return undefined

  return {
    key: 'favorites',
    label: $gettext('Favorites'),
    entries: favorites.value.map(item => ({
      key: `${item.dir}${item.name}`,
      title: item.name,
      description: configFavoriteDir(item) || undefined,
      path: configFavoriteRoute(item),
      icon: markRaw(FileOutlined),
    })),
  }
})

const staticSections = computed<HomeSection[]>(() => ([
  {
    key: 'management',
    label: $gettext('Management'),
    entries: [
      {
        title: $gettext('Manage Sites'),
        description: $gettext('Create, edit, and operate websites with guided workflows.'),
        path: '/sites',
      },
      {
        title: $gettext('Manage Configs'),
        description: $gettext('Browse and edit Nginx configuration files and directories.'),
        path: '/config',
      },
      {
        title: $gettext('Certificates'),
        description: $gettext('Issue, import, and maintain certificates and ACME accounts.'),
        path: '/certificates',
      },
      {
        title: $gettext('DNS'),
        description: $gettext('Manage DNS credentials, zones, groups, and dynamic updates.'),
        path: '/dns',
      },
      {
        title: $gettext('Upstream'),
        description: $gettext('Configure upstream backends and proxy target behaviors.'),
        path: '/upstream',
      },
      {
        title: $gettext('Manage Streams'),
        description: $gettext('Operate TCP/UDP stream proxy configurations and status.'),
        path: '/streams',
      },
    ],
  },
  {
    key: 'observability',
    label: $gettext('Observability'),
    entries: [
      {
        title: $gettext('Dashboard'),
        description: $gettext('View server, Nginx, and site-level performance metrics.'),
        path: '/dashboard',
      },
      {
        title: $gettext('Nginx Log'),
        description: $gettext('Inspect access and error logs with structured analysis tools.'),
        path: '/nginx_log',
      },
      {
        title: $gettext('Notifications'),
        description: $gettext('Track system events and message delivery status.'),
        path: '/notifications',
      },
    ],
  },
  {
    key: 'system',
    label: $gettext('System'),
    entries: [
      {
        title: $gettext('Nodes'),
        description: $gettext('Manage cluster nodes and synchronization targets.'),
        path: '/nodes',
      },
      {
        title: $gettext('Preference'),
        description: $gettext('Adjust application behavior, integrations, and auth settings.'),
        path: '/preference',
      },
      {
        title: $gettext('Backup'),
        description: $gettext('Create backups, schedule auto backup, and restore data.'),
        path: '/backup',
      },
      {
        title: $gettext('System'),
        description: $gettext('Review system health, upgrade status, and component details.'),
        path: '/system',
      },
      {
        title: $gettext('Terminal'),
        description: $gettext('Access integrated terminal sessions for maintenance tasks.'),
        path: '/terminal',
      },
    ],
  },
] satisfies HomeSection[]).map(section => ({
  ...section,
  entries: section.entries.map(item => ({ ...item, icon: resolveRouteIcon(item.path) })),
})))

const sections = computed<HomeSection[]>(() => (
  favoriteSection.value
    ? [favoriteSection.value, ...staticSections.value]
    : staticSections.value
))
</script>

<template>
  <div class="home">
    <section
      v-for="section in sections"
      :key="section.key"
      class="home-section"
      :aria-labelledby="`home-section-${section.key}`"
    >
      <h2
        :id="`home-section-${section.key}`"
        class="home-section-title"
      >
        {{ section.label }}
      </h2>

      <EntryGrid :entries="section.entries" />
    </section>
  </div>
</template>

<style lang="less" scoped>
.home {
  display: flex;
  flex-direction: column;
  gap: 32px;
}

.home-section-title {
  margin: 0 0 12px;
  font-size: 13px;
  font-weight: 600;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: var(--ant-color-text-secondary);
}
</style>
