import type { IndexProgress } from '../indexing/components/IndexProgressBar.vue'
import { useWebSocketEventBus } from '@/composables/useWebSocketEventBus'

export interface IndexProgressData {
  log_path: string
  progress: number
  stage: string
  status: string
  elapsed_time: number
  estimated_remain: number
}

export interface IndexCompleteData {
  log_path: string
  success: boolean
  duration: number
  total_lines: number
  indexed_size: number
  error?: string
}

export function useIndexProgress() {
  const { subscribe } = useWebSocketEventBus()

  // Store progress for each log file
  const progressMap = ref<Map<string, IndexProgress>>(new Map())

  // Global indexing state
  const isGlobalIndexing = ref(false)
  const globalProgress = ref<{
    totalFiles: number
    completedFiles: number
    currentFile?: string
    progress: number
  }>({
    totalFiles: 0,
    completedFiles: 0,
    progress: 0,
  })

  // Subscribe to progress events
  subscribe<IndexProgressData>('nginx_log_index_progress', data => {
    const progress: IndexProgress = {
      logPath: data.log_path,
      progress: data.progress,
      stage: data.stage,
      status: data.status,
      elapsedTime: data.elapsed_time,
      estimatedRemain: data.estimated_remain,
    }

    progressMap.value.set(data.log_path, progress)

    // Update global progress
    updateGlobalProgress()
  })

  // Subscribe to completion events
  subscribe<IndexCompleteData>('nginx_log_index_complete', data => {
    if (data.success) {
      // Keep progress for a short time to show completion, then remove
      setTimeout(() => {
        progressMap.value.delete(data.log_path)
        updateGlobalProgress()
      }, 3000)
    }
    else {
      // Show error state
      const errorProgress: IndexProgress = {
        logPath: data.log_path,
        progress: 0,
        stage: 'error',
        status: 'error',
        elapsedTime: data.duration,
        estimatedRemain: 0,
      }
      progressMap.value.set(data.log_path, errorProgress)

      // Remove error state after delay
      setTimeout(() => {
        progressMap.value.delete(data.log_path)
        updateGlobalProgress()
      }, 5000)
    }
  })

  // Subscribe to processing status events for global state
  subscribe<{ nginx_log_indexing: boolean }>('processing_status', data => {
    isGlobalIndexing.value = data.nginx_log_indexing
    if (!data.nginx_log_indexing) {
      // Clear all progress when indexing stops
      progressMap.value.clear()
      updateGlobalProgress()
    }
  })

  function updateGlobalProgress() {
    const activeFiles = Array.from(progressMap.value.values())
    globalProgress.value.totalFiles = activeFiles.length
    globalProgress.value.completedFiles = activeFiles.filter(p => p.status === 'completed').length

    if (activeFiles.length > 0) {
      const currentFile = activeFiles.find(p => p.status === 'running')
      globalProgress.value.currentFile = currentFile?.logPath

      // Calculate average progress
      const totalProgress = activeFiles.reduce((sum, p) => sum + p.progress, 0)
      globalProgress.value.progress = totalProgress / activeFiles.length
    }
    else {
      globalProgress.value.currentFile = undefined
      globalProgress.value.progress = 0
    }
  }

  function getProgressForFile(logPath: string): IndexProgress | undefined {
    return progressMap.value.get(logPath)
  }

  function isFileIndexing(logPath: string): boolean {
    const progress = progressMap.value.get(logPath)
    return progress?.status === 'running'
  }

  function clearProgress(logPath: string) {
    progressMap.value.delete(logPath)
    updateGlobalProgress()
  }

  function clearAllProgress() {
    progressMap.value.clear()
    updateGlobalProgress()
  }

  return {
    // Reactive data
    progressMap: readonly(progressMap),
    isGlobalIndexing: readonly(isGlobalIndexing),
    globalProgress: readonly(globalProgress),

    // Methods
    getProgressForFile,
    isFileIndexing,
    clearProgress,
    clearAllProgress,
  }
}
