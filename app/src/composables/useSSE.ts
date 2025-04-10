import type { SSEvent } from 'sse.js'
import { SSE } from 'sse.js'
import { onUnmounted, shallowRef } from 'vue'

export interface SSEOptions {
  url: string
  token: string
  onMessage?: (data: any) => void
  onError?: () => void
  parseData?: boolean
  reconnectInterval?: number
}

/**
 * SSE 连接 Composable
 * 提供创建、管理和自动清理 SSE 连接的能力
 */
export function useSSE() {
  const sseInstance = shallowRef<SSE>()

  /**
   * 连接 SSE 服务
   */
  function connect(options: SSEOptions) {
    disconnect()

    const {
      url,
      token,
      onMessage,
      onError,
      parseData = true,
      reconnectInterval = 5000,
    } = options

    const sse = new SSE(url, {
      headers: {
        Authorization: token,
      },
    })

    // 处理消息
    sse.onmessage = (e: SSEvent) => {
      if (!e.data) {
        return
      }

      try {
        const parsedData = parseData ? JSON.parse(e.data) : e.data
        onMessage?.(parsedData)
      }
      catch (error) {
        console.error('Error parsing SSE message:', error)
      }
    }

    // 处理错误并重连
    sse.onerror = () => {
      onError?.()

      // 重连逻辑
      setTimeout(() => {
        connect(options)
      }, reconnectInterval)
    }

    sseInstance.value = sse
    return sse
  }

  /**
   * 断开 SSE 连接
   */
  function disconnect() {
    if (sseInstance.value) {
      sseInstance.value.close()
      sseInstance.value = undefined
    }
  }

  // 组件卸载时自动断开连接
  onUnmounted(() => {
    disconnect()
  })

  return {
    connect,
    disconnect,
    sseInstance,
  }
}
