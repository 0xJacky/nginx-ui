<script setup lang="ts">
import type { OSStatItem } from '../types'
import type { DashboardAnalytics } from '@/api/nginx_log'
import { Card, Table } from 'ant-design-vue'

defineProps<{
  dashboardData: DashboardAnalytics | null
  loading: boolean
}>()

const osColumns = [
  {
    title: () => $gettext('Operating System'),
    dataIndex: 'os',
    key: 'os',
  },
  {
    title: () => $gettext('Count'),
    dataIndex: 'count',
    key: 'count',
    sorter: (a: OSStatItem, b: OSStatItem) => a.count - b.count,
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
  <Card :title="$gettext('Operating System Statistics')" size="small" :loading="loading">
    <Table
      v-if="dashboardData"
      :columns="osColumns"
      :data-source="dashboardData?.operating_systems?.slice(0, 10) || []"
      :pagination="false"
      row-key="os"
      size="small"
      :scroll="{ y: 200 }"
    />
  </Card>
</template>
