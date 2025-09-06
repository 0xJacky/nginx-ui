<script setup lang="ts">
import { EyeInvisibleOutlined, EyeOutlined, ReloadOutlined } from '@ant-design/icons-vue'
import { message } from 'ant-design-vue'
import system from '@/api/system'

interface Props {
  title?: string
  initialVisible?: boolean
  debugData?: Record<string, unknown>
}

const props = withDefaults(defineProps<Props>(), {
  title: 'Debug Panel',
  initialVisible: false,
  debugData: () => ({}),
})

const isDev = import.meta.env.DEV
const envMode = import.meta.env.MODE
const isVisible = ref(props.initialVisible)
const pid = ref<number | null>(null)
const isRestarting = ref(false)

function togglePanel() {
  isVisible.value = !isVisible.value
}

async function fetchPID() {
  try {
    const stats = await system.getProcessStats()
    pid.value = stats.pid
  }
  catch (error) {
    console.error('Failed to fetch PID:', error)
  }
}

async function restartSystem() {
  try {
    isRestarting.value = true
    await system.restart()
    message.success('System restart initiated')
  }
  catch (error) {
    console.error('Failed to restart system:', error)
    message.error('Failed to restart system')
  }
  finally {
    isRestarting.value = false
  }
}

// Fetch PID when component mounts and panel becomes visible
onMounted(() => {
  if (isVisible.value) {
    fetchPID()
  }
})

watch(isVisible, newVisible => {
  if (newVisible && pid.value === null) {
    fetchPID()
  }
})

// Ensure component only works in development
if (!isDev) {
  console.warn('DevDebugPanel should only be used in development environment')
}

// Prevent rendering in production
const shouldRender = computed(() => isDev)
</script>

<template>
  <div v-if="shouldRender" class="dev-debug-panel">
    <div class="debug-toggle">
      <AButton
        type="primary"
        size="small"
        :icon="isVisible ? h(EyeInvisibleOutlined) : h(EyeOutlined)"
        @click="togglePanel"
      >
        {{ isVisible ? 'Hide Debug' : 'Show Debug' }}
      </AButton>
    </div>

    <div v-if="isVisible" class="debug-content">
      <div class="debug-header">
        <h4>🐛 Dev Debug Panel</h4>
        <span class="debug-env">{{ envMode }}</span>
      </div>

      <div class="debug-body">
        <div class="debug-item">
          <span class="debug-label">PID:</span>
          <span class="debug-value">{{ pid ?? 'Loading...' }}</span>
          <AButton
            size="small"
            type="link"
            :icon="h(ReloadOutlined)"
            title="Refresh PID"
            @click="fetchPID"
          />
        </div>

        <div class="debug-item">
          <AButton
            size="small"
            type="primary"
            danger
            :loading="isRestarting"
            :icon="h(ReloadOutlined)"
            @click="restartSystem"
          >
            {{ isRestarting ? 'Restarting...' : 'Restart System' }}
          </AButton>
        </div>

        <slot :debug-data="debugData" />
      </div>
    </div>
  </div>
</template>

<style scoped lang="less">
.dev-debug-panel {
  position: fixed !important;
  bottom: 20px !important;
  right: 20px !important;
  top: auto !important;
  z-index: 9999;
  font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
}

.debug-toggle {
  margin-bottom: 8px;
}

.debug-content {
  background: rgba(0, 0, 0, 0.9);
  border: 2px solid #1890ff;
  border-radius: 8px;
  padding: 12px;
  min-width: 300px;
  max-width: 500px;
  color: #fff;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3);
  backdrop-filter: blur(10px);
}

.debug-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
  padding-bottom: 8px;
  border-bottom: 1px solid #333;

  h4 {
    margin: 0;
    color: #1890ff;
    font-size: 14px;
  }

  .debug-env {
    background: #52c41a;
    color: #fff;
    padding: 2px 6px;
    border-radius: 4px;
    font-size: 10px;
    text-transform: uppercase;
    font-weight: bold;
  }
}

.debug-body {
  font-size: 12px;
  line-height: 1.4;

  :deep(pre) {
    background: rgba(255, 255, 255, 0.1);
    padding: 8px;
    border-radius: 4px;
    overflow: auto;
    margin: 4px 0;
  }

  :deep(.debug-item) {
    margin: 8px 0;
    padding: 4px 0;
    border-bottom: 1px solid #333;
    display: flex;
    align-items: center;
    gap: 8px;

    &:last-child {
      border-bottom: none;
    }
  }

  :deep(.debug-label) {
    color: #1890ff;
    font-weight: bold;
    margin-right: 8px;
  }

  :deep(.debug-value) {
    color: #52c41a;
    flex: 1;
  }

}

.dark .debug-content {
  background: rgba(20, 20, 20, 0.95);
  border-color: #1890ff;
  color: #fff;
}
</style>
