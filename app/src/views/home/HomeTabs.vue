<script setup lang="ts">
import EntryGrid from '@/components/EntryGrid/EntryGrid.vue'

interface EntryItem {
  title: string
  description: string
  path: string
}

interface HomeTab {
  key: string
  label: string
  entries: EntryItem[]
}

const HOME_TAB_STORAGE_KEY = 'nginx-ui.home.active-tab'

const activeTab = ref('management')

const tabs = computed<HomeTab[]>(() => [
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
    key: 'Observability',
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
    key: 'System',
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
])

function getAvailableTabKeys() {
  return tabs.value.map(tab => tab.key)
}

function restoreActiveTab() {
  if (typeof window === 'undefined')
    return

  const stored = localStorage.getItem(HOME_TAB_STORAGE_KEY)
  if (!stored)
    return

  if (getAvailableTabKeys().includes(stored))
    activeTab.value = stored
}

watch(activeTab, value => {
  if (typeof window === 'undefined')
    return

  localStorage.setItem(HOME_TAB_STORAGE_KEY, value)
})

watch(tabs, () => {
  const available = getAvailableTabKeys()
  if (!available.includes(activeTab.value)) {
    activeTab.value = available[0] ?? 'management'
  }
})

onMounted(() => {
  restoreActiveTab()
})
</script>

<template>
  <ATabs v-model:active-key="activeTab" class="home-tabs" :animated="false">
    <ATabPane
      v-for="tab in tabs"
      :key="tab.key"
      :tab="tab.label"
    >
      <EntryGrid :entries="tab.entries" />
    </ATabPane>
  </ATabs>
</template>

<style scoped>
.home-tabs :deep(.ant-tabs-nav) {
  margin-bottom: 16px;
}
</style>
