<script setup lang="ts">
import { FileOutlined } from '@ant-design/icons-vue'
import { useRouteQuery } from '@vueuse/router'
import FooterToolBar from '@/components/FooterToolbar'
import DashboardViewer from './dashboard/DashboardViewer.vue'
import RawLogViewer from './raw/RawLogViewer.vue'
import StructuredLogViewer from './structured/StructuredLogViewer.vue'

// Route and router
const route = useRoute()
const router = useRouter()

// Setup log control data based on route params
const logPath = computed(() => route.query.path?.toString() ?? '')
const logType = computed(() => {
  if (route.path.indexOf('access') > 0)
    return 'access'
  return route.path.indexOf('error') > 0 ? 'error' : 'site'
})

const viewMode = useRouteQuery<'raw' | 'structured' | 'dashboard'>('view', 'structured')

// Check if this is an error log
const isErrorLog = computed(() => {
  return logType.value === 'error' || logPath.value.includes('error.log') || logPath.value.includes('error_log')
})

const autoRefresh = ref(true)
</script>

<template>
  <ACard
    :title="$gettext('Nginx Log')"
    :bordered="false"
  >
    <!-- Log Path Header -->
    <div v-if="logPath" class="mb-4 px-2 py-1.5 bg-gray-50 dark:bg-gray-800 rounded text-xs text-gray-500 dark:text-gray-400">
      <FileOutlined class="mr-2" />
      <span class="font-mono">{{ logPath }}</span>
    </div>

    <template #extra>
      <div class="flex items-center gap-4">
        <!-- View Mode Toggle (hide for error logs) -->
        <div v-if="!isErrorLog" class="flex items-center">
          <ASegmented
            v-model:value="viewMode"
            :options="[
              { label: $gettext('Structured'), value: 'structured' },
              { label: $gettext('Dashboard'), value: 'dashboard' },
              { label: $gettext('Raw'), value: 'raw' },
            ]"
          />
        </div>

        <!-- Auto Refresh (only for raw mode) -->
        <div v-if="viewMode === 'raw'" class="flex items-center">
          <span class="mr-2">{{ $gettext('Auto Refresh') }}</span>
          <ASwitch v-model:checked="autoRefresh" />
        </div>
      </div>
    </template>

    <!-- Raw Log View -->
    <RawLogViewer
      v-if="viewMode === 'raw'"
      :log-path="logPath"
      :log-type="logType"
      :auto-refresh="autoRefresh"
    />

    <!-- Structured Log View -->
    <StructuredLogViewer
      v-else-if="viewMode === 'structured'"
      :log-path="logPath"
    />

    <!-- Dashboard View -->
    <DashboardViewer
      v-else-if="viewMode === 'dashboard'"
      :log-path="logPath"
    />

    <FooterToolBar v-if="logPath">
      <AButton @click="router.go(-1)">
        {{ $gettext('Back') }}
      </AButton>
    </FooterToolBar>
  </ACard>
</template>
