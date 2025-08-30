<script setup lang="ts">
import type { CSSProperties } from 'vue'
import { LoadingOutlined } from '@ant-design/icons-vue'
import { computed } from 'vue'
import { useIndexProgress } from '../composables/useIndexProgress'
import IndexProgressBar from '../indexing/components/IndexProgressBar.vue'

interface IndexStatusDetails {
  available: boolean
  index_status: string
  message?: string
}

const props = defineProps<{
  logPath?: string
  size?: 'small' | 'default' | 'large'
  indexStatus?: IndexStatusDetails
}>()

// Index progress tracking
const { getProgressForFile, isFileIndexing } = useIndexProgress()
const indexProgress = computed(() => props.logPath ? getProgressForFile(props.logPath) : null)
const isCurrentFileIndexing = computed(() => props.logPath ? isFileIndexing(props.logPath) : false)

// Status-based loading message and icon
const statusInfo = computed(() => {
  const status = props.indexStatus?.index_status

  switch (status) {
    case 'indexing':
      return {
        icon: LoadingOutlined,
        message: $gettext('Indexing logs...'),
        color: 'text-blue-500',
        showProgress: true,
      }
    case 'indexed':
      return {
        icon: LoadingOutlined,
        message: $gettext('Loading...'),
        color: 'text-blue-500',
        showProgress: false,
      }
    case 'ready':
      return {
        icon: LoadingOutlined,
        message: $gettext('Loading...'),
        color: 'text-green-500',
        showProgress: false,
      }
    case 'queued':
      return {
        icon: LoadingOutlined,
        message: $gettext('Queued for indexing...'),
        color: 'text-orange-500',
        showProgress: false,
      }
    case 'partial':
      return {
        icon: LoadingOutlined,
        message: $gettext('Partially indexed, resuming...'),
        color: 'text-blue-500',
        showProgress: true,
      }
    case 'error':
      return {
        icon: LoadingOutlined,
        message: $gettext('Index failed, please try rebuilding'),
        color: 'text-red-500',
        showProgress: false,
      }
    case 'not_indexed':
      return {
        icon: LoadingOutlined,
        message: $gettext('Log file not indexed yet'),
        color: 'text-gray-500',
        showProgress: false,
      }
    default:
      // Fallback for active indexing check
      if (isCurrentFileIndexing.value) {
        return {
          icon: LoadingOutlined,
          message: $gettext('Indexing...'),
          color: 'text-blue-500',
          showProgress: true,
        }
      }
      return {
        icon: LoadingOutlined,
        message: $gettext('Loading...'),
        color: 'text-blue-500',
        showProgress: false,
      }
  }
})

const iconClass = computed(() => {
  const baseColor = statusInfo.value.color || 'text-blue-500'
  switch (props.size) {
    case 'small':
      return `text-lg ${baseColor}`
    case 'large':
      return `text-4xl ${baseColor}`
    default:
      return `text-2xl ${baseColor}`
  }
})

const containerStyle = computed((): CSSProperties => {
  let height = '50vh' // Default responsive height
  let padding = '40px'

  switch (props.size) {
    case 'small':
      height = '30vh'
      padding = '20px'
      break
    case 'large':
      height = '70vh'
      padding = '60px'
      break
  }

  return {
    minHeight: height,
    padding,
    display: 'flex',
    flexDirection: 'column' as const,
    justifyContent: 'center',
    alignItems: 'center',
  }
})
</script>

<template>
  <div :style="containerStyle">
    <!-- Status Icon -->
    <component :is="statusInfo.icon" :class="iconClass" />

    <!-- Progress Bar (only show when actively indexing or if progress data exists) -->
    <div v-if="(statusInfo.showProgress && indexProgress) || indexProgress" class="mt-4 flex flex-col items-center">
      <div class="max-w-75 w-full">
        <IndexProgressBar
          :progress="indexProgress"
          size="small"
        />
      </div>
    </div>

    <!-- Status Message -->
    <p class="mt-4">
      {{ statusInfo.message }}
    </p>
  </div>
</template>
