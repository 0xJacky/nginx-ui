<script setup lang="ts">
import { useNginxPerformance } from '@/composables/useNginxPerformance'
import { useSSE } from '@/composables/useSSE'
import { NginxStatus } from '@/constants'
import { useUserStore } from '@/pinia'
import { useGlobalStore } from '@/pinia/moudule/global'
import { ClockCircleOutlined, ReloadOutlined } from '@ant-design/icons-vue'
import { storeToRefs } from 'pinia'
import ConnectionMetricsCard from './components/ConnectionMetricsCard.vue'
import PerformanceStatisticsCard from './components/PerformanceStatisticsCard.vue'
import PerformanceTablesCard from './components/PerformanceTablesCard.vue'
import ProcessDistributionCard from './components/ProcessDistributionCard.vue'
import ResourceUsageCard from './components/ResourceUsageCard.vue'

// 全局状态
const global = useGlobalStore()
const { nginxStatus: status } = storeToRefs(global)
const { token } = storeToRefs(useUserStore())

// 使用性能数据composable
const {
  loading,
  nginxInfo,
  error,
  formattedUpdateTime,
  updateLastUpdateTime,
  fetchInitialData,
} = useNginxPerformance()

// SSE 连接
const { connect, disconnect } = useSSE()

// 连接SSE
function connectSSE() {
  disconnect()
  loading.value = true

  connect({
    url: 'api/nginx/detailed_status/stream',
    token: token.value,
    onMessage: data => {
      loading.value = false

      if (data.running) {
        nginxInfo.value = data.info
        updateLastUpdateTime()
      }
      else {
        error.value = data.message || $gettext('Nginx is not running')
      }
    },
    onError: () => {
      error.value = $gettext('Connection error, trying to reconnect...')

      // 如果连接失败，尝试使用传统方式获取数据
      setTimeout(() => {
        fetchInitialData()
      }, 5000)
    },
  })
}

// 手动刷新数据
function refreshData() {
  fetchInitialData().then(connectSSE)
}

// 组件挂载时初始化连接
onMounted(() => {
  fetchInitialData().then(connectSSE)
})
</script>

<template>
  <div>
    <!-- 顶部操作栏 -->
    <div class="mb-4 mx-6 md:mx-0 flex flex-wrap justify-between items-center">
      <div class="flex items-center">
        <ABadge :status="status === NginxStatus.Running ? 'success' : 'error'" />
        <span class="font-medium">{{ status === NginxStatus.Running ? $gettext('Nginx is running') : $gettext('Nginx is not running') }}</span>
      </div>
      <div class="flex items-center">
        <ClockCircleOutlined class="mr-1 text-gray-500" />
        <span class="mr-4 text-gray-500 text-sm text-nowrap">{{ $gettext('Last update') }}: {{ formattedUpdateTime }}</span>
        <AButton type="text" size="small" :loading="loading" @click="refreshData">
          <template #icon>
            <ReloadOutlined />
          </template>
        </AButton>
      </div>
    </div>

    <!-- Nginx 状态提示 -->
    <AAlert
      v-if="status !== NginxStatus.Running"
      class="mb-4"
      type="warning"
      show-icon
      :message="$gettext('Nginx is not running')"
      :description="$gettext('Cannot get performance data in this state')"
    />

    <!-- 错误提示 -->
    <AAlert
      v-if="error"
      class="mb-4"
      type="error"
      show-icon
      :message="$gettext('Get data failed')"
      :description="error"
    />

    <!-- 加载中状态 -->
    <ASpin :spinning="loading" :tip="$gettext('Loading data...')">
      <div v-if="!nginxInfo && !loading && !error" class="text-center py-8">
        <AEmpty :description="$gettext('No data')" />
      </div>

      <div v-if="nginxInfo" class="performance-dashboard">
        <!-- 顶部性能指标卡片 -->
        <ARow :gutter="[16, 16]" class="mb-4">
          <ACol :span="24">
            <ACard :title="$gettext('Performance Metrics')" :bordered="false">
              <PerformanceStatisticsCard :nginx-info="nginxInfo" />
            </ACard>
          </ACol>
        </ARow>

        <!-- 指标卡片 -->
        <ConnectionMetricsCard :nginx-info="nginxInfo" class="mb-4" />

        <!-- 资源监控 -->
        <ARow :gutter="[16, 16]" class="mb-4">
          <!-- CPU和内存使用 -->
          <ACol :xs="24" :md="12">
            <ResourceUsageCard :nginx-info="nginxInfo" />
          </ACol>
          <!-- 进程分布 -->
          <ACol :xs="24" :md="12">
            <ProcessDistributionCard :nginx-info="nginxInfo" />
          </ACol>
        </ARow>

        <!-- 性能指标表格 -->
        <ARow :gutter="[16, 16]" class="mb-4">
          <ACol :span="24">
            <PerformanceTablesCard :nginx-info="nginxInfo" />
          </ACol>
        </ARow>
      </div>
    </ASpin>
  </div>
</template>
