<script setup lang="ts">
import type { DashboardAnalytics } from '@/api/nginx_log'
import { Card, Col, Row, Statistic } from 'ant-design-vue'

defineProps<{
  dashboardData: DashboardAnalytics | null
}>()
</script>

<template>
  <Row v-if="dashboardData" :gutter="16" class="mb-4">
    <Col :span="6">
      <Card size="small">
        <Statistic
          :title="$gettext('Total UV')"
          :value="dashboardData.summary.total_uv.toLocaleString()"
          :value-style="{ color: '#52c41a' }"
        />
      </Card>
    </Col>
    <Col :span="6">
      <Card size="small">
        <Statistic
          :title="$gettext('Total PV')"
          :value="dashboardData.summary.total_pv.toLocaleString()"
          :value-style="{ color: '#1890ff' }"
        />
      </Card>
    </Col>
    <Col :span="6">
      <Card size="small">
        <Statistic
          :title="$gettext('Avg Daily UV')"
          :value="Math.round(dashboardData.summary.avg_daily_uv).toLocaleString()"
          :value-style="{ color: '#faad14' }"
        />
      </Card>
    </Col>
    <Col :span="6">
      <Card size="small">
        <Statistic
          :title="$gettext('Peak Hour')"
          :value="`${dashboardData.summary.peak_hour}:00`"
          :value-style="{ color: '#722ed1' }"
        />
      </Card>
    </Col>
  </Row>
</template>
