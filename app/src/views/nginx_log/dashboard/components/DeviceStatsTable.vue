<script setup lang="ts">
import type { DeviceStatItem } from '../types'
import type { DashboardAnalytics } from '@/api/nginx_log'

defineProps<{
  dashboardData: DashboardAnalytics | null
  loading: boolean
}>()

const deviceColumns = [
  {
    title: () => $gettext('Device Type'),
    dataIndex: 'device',
    key: 'device',
  },
  {
    title: () => $gettext('Count'),
    dataIndex: 'count',
    key: 'count',
    sorter: (a: DeviceStatItem, b: DeviceStatItem) => a.count - b.count,
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
  <ACard :title="$gettext('Device Statistics')" size="small" :loading="loading">
    <ATable
      v-if="dashboardData"
      :columns="deviceColumns"
      :data-source="dashboardData?.devices?.slice(0, 10) || []"
      :pagination="false"
      row-key="device"
      size="small"
      :scroll="{ y: 200 }"
    />
  </ACard>
</template>
