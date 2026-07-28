<script setup lang="ts">
import type { DashboardAnalytics } from '@/api/nginx_log'
import { Card, Col, Row, Statistic } from 'ant-design-vue'
import { bytesToSize } from '@/lib/helper'

const props = defineProps<{
  dashboardData: DashboardAnalytics | null
}>()

// Request rates span several orders of magnitude between a quiet vhost and a
// busy one, so keep two significant decimals until the value is large enough
// for them to be noise.
function formatQps(value: number) {
  if (!value)
    return '0'
  if (value >= 100)
    return value.toFixed(0)
  if (value >= 1)
    return value.toFixed(2)
  return value.toFixed(3)
}

const cards = computed(() => {
  const summary = props.dashboardData?.summary
  if (!summary)
    return []

  return [
    { key: 'pv', title: $gettext('Total PV'), value: summary.total_pv.toLocaleString(), color: '#1890ff' },
    { key: 'uv', title: $gettext('Total UV'), value: summary.total_uv.toLocaleString(), color: '#52c41a' },
    { key: 'traffic', title: $gettext('Total Traffic'), value: bytesToSize(summary.total_traffic), color: '#13c2c2' },
    { key: 'peak_hour', title: $gettext('Peak Hour'), value: `${summary.peak_hour}:00`, color: '#722ed1' },
    { key: 'avg_qps', title: $gettext('Avg QPS'), value: formatQps(summary.avg_qps), color: '#eb2f96' },
    { key: 'peak_qps', title: $gettext('Peak QPS'), value: formatQps(summary.peak_qps), color: '#f5222d' },
    { key: 'avg_daily_pv', title: $gettext('Avg Daily PV'), value: Math.round(summary.avg_daily_pv).toLocaleString(), color: '#2f54eb' },
    { key: 'avg_daily_uv', title: $gettext('Avg Daily UV'), value: Math.round(summary.avg_daily_uv).toLocaleString(), color: '#faad14' },
  ]
})
</script>

<template>
  <Row v-if="cards.length" :gutter="[16, 16]" class="mb-4">
    <Col
      v-for="card in cards"
      :key="card.key"
      :xs="12"
      :sm="12"
      :md="6"
    >
      <Card size="small">
        <Statistic
          :title="card.title"
          :value="card.value"
          :value-style="{ color: card.color }"
        />
      </Card>
    </Col>
  </Row>
</template>
