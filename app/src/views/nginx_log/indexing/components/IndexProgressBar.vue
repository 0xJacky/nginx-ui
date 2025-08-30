<script setup lang="ts">
export interface IndexProgress {
  logPath: string
  progress: number
  stage: string
  status: string
  elapsedTime: number
  estimatedRemain: number
}

const props = defineProps<{
  progress: IndexProgress | null
  size?: 'small' | 'default'
}>()

function formatTime(milliseconds: number): string {
  const seconds = Math.floor(milliseconds / 1000)
  const minutes = Math.floor(seconds / 60)
  const hours = Math.floor(minutes / 60)

  if (hours > 0) {
    return `${hours}h ${minutes % 60}m ${seconds % 60}s`
  }
  if (minutes > 0) {
    return `${minutes}m ${seconds % 60}s`
  }
  return `${seconds}s`
}

const progressColor = computed(() => {
  if (!props.progress)
    return '#1890ff'

  switch (props.progress.status) {
    case 'error':
      return '#ff4d4f'
    case 'completed':
      return '#52c41a'
    default:
      return '#1890ff'
  }
})

const stageText = computed(() => {
  if (!props.progress)
    return ''

  switch (props.progress.stage) {
    case 'scanning':
      return $gettext('Scanning')
    case 'indexing':
      return $gettext('Indexing')
    case 'stats':
      return $gettext('Computing Statistics')
    default:
      return props.progress.stage
  }
})
</script>

<template>
  <div v-if="progress" class="index-progress">
    <div class="progress-info">
      <div class="info-left">
        <ATag :color="progressColor" size="small" class="stage-tag">
          {{ stageText }}
        </ATag>
      </div>
      <div class="info-right text-gray-600 dark:text-gray-400">
        <span class="time-elapsed text-gray-500 dark:text-gray-400">{{ formatTime(progress.elapsedTime) }}</span>
        <span v-if="progress.estimatedRemain > 0" class="time-eta text-green-500 dark:text-green-400">
          ETA {{ formatTime(progress.estimatedRemain) }}
        </span>
      </div>
    </div>

    <div class="progress-container">
      <AProgress
        :percent="Math.round(progress.progress)"
        size="small"
        :stroke-color="progressColor"
        :show-info="false"
        class="compact-progress"
      />
      <span class="percent-info text-blue-500 dark:text-blue-400">
        {{ Math.round(progress.progress) }}%
      </span>
    </div>
  </div>
</template>

<style scoped>
.index-progress {
  width: 100%;
  min-width: 200px;
  max-width: 300px;
  padding: 2px 0;
}

.progress-info {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 2px;
  font-size: 12px;
}

.info-left {
  display: flex;
  align-items: center;
  gap: 6px;
}

.info-right {
  display: flex;
  align-items: center;
  gap: 4px;
  white-space: nowrap;
  font-size: 11px;
}

.progress-container {
  display: flex;
  align-items: center;
  gap: 8px;
  height: 20px;
  margin-top: 2px;
}

.stage-tag {
  font-size: 11px !important;
  line-height: 1.2 !important;
  padding: 1px 4px !important;
}

.progress-text {
  font-size: 11px;
  font-weight: 500;
}

.percent-info {
  font-size: 11px;
  font-weight: 600;
  min-width: 30px;
  text-align: right;
  display: flex;
  align-items: center;
  justify-content: flex-end;
  height: 100%;
}

.time-elapsed {
  color: #999;
}

.time-eta {
  color: #52c41a;
  font-weight: 500;
}

.compact-progress {
  flex: 1;
  display: flex;
  align-items: center;
  margin: 0 !important;
}

.compact-progress :deep(.ant-progress-inner) {
  height: 6px !important;
}

.compact-progress :deep(.ant-progress-outer) {
  display: flex;
  align-items: center;
  height: 100%;
  margin: 0 !important;
}

.compact-progress :deep(.ant-progress-line) {
  margin: 0 !important;
}

.compact-progress :deep(.ant-progress-status-normal) {
  margin: 0 !important;
}

.compact-progress :deep(.ant-progress-small) {
  margin: 0 !important;
}

.compact-progress :deep(.ant-progress) {
  margin: 0 !important;
  padding: 0 !important;
}

.compact-progress :deep(.ant-progress *) {
  margin: 0 !important;
}
</style>
