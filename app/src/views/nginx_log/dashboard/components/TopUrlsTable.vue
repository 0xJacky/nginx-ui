<script setup lang="ts">
import type { URLStatItem } from '../types'
import type { DashboardAnalytics } from '@/api/nginx_log'
import { Card, Table } from 'ant-design-vue'

defineProps<{
  dashboardData: DashboardAnalytics | null
  loading: boolean
}>()

const urlColumns = [
  {
    title: () => $gettext('URL'),
    dataIndex: 'url',
    key: 'url',
    ellipsis: true,
  },
  {
    title: () => $gettext('Visits'),
    dataIndex: 'visits',
    key: 'visits',
    sorter: (a: URLStatItem, b: URLStatItem) => a.visits - b.visits,
    width: 100,
    customRender: ({ text }: { text: number }) => text.toLocaleString(),
  },
  {
    title: () => $gettext('Percentage'),
    dataIndex: 'percent',
    key: 'percent',
    customRender: ({ text }: { text: number }) => `${text.toFixed(2)}%`,
    width: 120,
  },
]
</script>

<template>
  <Card :title="$gettext('TOP 10 URLs')" size="small" class="mb-4" :loading="loading">
    <Table
      v-if="dashboardData"
      :columns="urlColumns"
      :data-source="dashboardData.top_urls"
      :pagination="false"
      row-key="url"
      size="small"
      :scroll="{ y: 240 }"
    />
  </Card>
</template>
