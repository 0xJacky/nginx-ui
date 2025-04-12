import cacheIndex from '@/api/cache_index'
import { SSE } from 'sse.js'
import { useSSE } from './useSSE'

/**
 * Composable for monitoring cache index status
 * Provides a way to track indexing/scanning status through SSE
 */

export interface IndexStatus {
  isScanning: Ref<boolean>
}

/**
 * Setup SSE connection to monitor indexing status
 */
export function setupIndexStatus() {
  const { connect, disconnect, sseInstance } = useSSE()

  const isScanning = ref(false)

  disconnect()

  const sse = cacheIndex.index_status()

  if (sse instanceof SSE) {
    connect({
      url: '', // Not needed as we already have the SSE instance
      token: '', // Not needed as we already have the SSE instance
      onMessage: data => {
        isScanning.value = data.scanning
      },
      onError: () => {
        // Reconnection is handled by useSSE
      },
    })

    // Manually assign the SSE instance since we're using a pre-created one
    sseInstance.value = sse
  }

  provide('indexStatus', {
    isScanning,
  })
}

export function useIndexStatus(): IndexStatus {
  return inject<IndexStatus>('indexStatus')!
}
