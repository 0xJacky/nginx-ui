<script setup lang="ts">
import FooterToolBar from '@/components/FooterToolbar'
import RawLogViewer from './components/RawLogViewer.vue'
import StructuredLogViewer from './components/StructuredLogViewer.vue'

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

// Check if this is an error log
const isErrorLog = computed(() => {
  return logType.value === 'error' || logPath.value.includes('error.log') || logPath.value.includes('error_log')
})

// Reactive data - use reactive to handle computed changes
const viewMode = ref<'raw' | 'structured'>('structured')
const autoRefresh = ref(true)

// Set initial view mode and watch for error log changes
watchEffect(() => {
  if (isErrorLog.value) {
    viewMode.value = 'raw'
  }
})

// Watch for view mode changes
watch(viewMode, newMode => {
  if (newMode === 'structured') {
    autoRefresh.value = false
  }
})
</script>

<template>
  <ACard
    :title="$gettext('Nginx Log')"
    :bordered="false"
  >
    <template #extra>
      <div class="flex items-center gap-4">
        <!-- View Mode Toggle (hide for error logs) -->
        <div v-if="!isErrorLog" class="flex items-center">
          <ASegmented
            v-model:value="viewMode"
            :options="[
              { label: $gettext('Structured'), value: 'structured' },
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

    <FooterToolBar v-if="logPath">
      <AButton @click="router.go(-1)">
        {{ $gettext('Back') }}
      </AButton>
    </FooterToolBar>
  </ACard>
</template>
