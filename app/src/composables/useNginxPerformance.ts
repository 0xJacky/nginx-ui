import type { NginxPerformanceInfo } from '@/api/ngx'
import ngx from '@/api/ngx'
import { computed, ref } from 'vue'

export function useNginxPerformance() {
  const loading = ref(true)
  const nginxInfo = ref<NginxPerformanceInfo>()
  const error = ref<string>('')
  const lastUpdateTime = ref(new Date())

  // 更新刷新时间
  function updateLastUpdateTime() {
    lastUpdateTime.value = new Date()
  }

  // 格式化上次更新时间
  const formattedUpdateTime = computed(() => {
    return lastUpdateTime.value.toLocaleTimeString()
  })

  // 获取Nginx状态数据
  async function fetchInitialData() {
    loading.value = true
    error.value = ''

    try {
      const result = await ngx.detailed_status()
      nginxInfo.value = result.info
      updateLastUpdateTime()
    }
    catch (e) {
      if (e instanceof Error) {
        error.value = e.message
      }
      else {
        error.value = $gettext('Get data failed')
      }
    }
    finally {
      loading.value = false
    }
  }

  return {
    loading,
    nginxInfo,
    error,
    lastUpdateTime,
    formattedUpdateTime,
    updateLastUpdateTime,
    fetchInitialData,
  }
}
