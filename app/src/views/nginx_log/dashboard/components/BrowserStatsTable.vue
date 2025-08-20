<script setup lang="ts">
import type { BrowserStatItem } from '../types'
import type { DashboardAnalytics } from '@/api/nginx_log'
import { Card, Table } from 'ant-design-vue'

defineProps<{
  dashboardData: DashboardAnalytics | null
  loading: boolean
}>()

const browserColumns = [
  {
    title: () => $gettext('Browser'),
    dataIndex: 'browser',
    key: 'browser',
  },
  {
    title: () => $gettext('Count'),
    dataIndex: 'count',
    key: 'count',
    sorter: (a: BrowserStatItem, b: BrowserStatItem) => a.count - b.count,
    width: 80,
    customRender: ({ text }: { text: number }) => text.toLocaleString(),
  },
  {
    title: () => $gettext('Percentage'),
    dataIndex: 'percent',
    key: 'percent',
    customRender: ({ text }: { text: number }) => `${text.toFixed(2)}%`,
    width: 100,
  },
]
</script>

<template>
  <Card :title="$gettext('Browser Statistics')" size="small" :loading="loading">
    <Table
      v-if="dashboardData"
      :columns="browserColumns"
      :data-source="dashboardData.browsers.slice(0, 10)"
      :pagination="false"
      row-key="browser"
      size="small"
      :scroll="{ y: 200 }"
    />
  </Card>
</template>
