import type { NginxPerformanceInfo, NginxPerformanceResponse } from '@/api/ngx'
import ngx from '@/api/ngx'
import { formatDateTime } from '@/lib/helper'

export function useNginxPerformance() {
  const loading = ref(false)
  const error = ref('')
  const nginxInfo = ref<NginxPerformanceInfo | null>(null)
  const lastUpdateTime = ref<string>('')

  // stub_status availability
  const stubStatusEnabled = ref(false)
  const stubStatusLoading = ref(false)
  const stubStatusError = ref('')

  // Format the last update time
  const formattedUpdateTime = computed(() => {
    if (!lastUpdateTime.value)
      return $gettext('Unknown')
    return formatDateTime(lastUpdateTime.value)
  })

  // Update the last update time
  function updateLastUpdateTime() {
    lastUpdateTime.value = new Date().toISOString()
  }

  function applyPerformanceData(response: NginxPerformanceResponse) {
    loading.value = false
    stubStatusEnabled.value = response.stub_status_enabled
    error.value = response.error || (response.running ? '' : response.message || $gettext('Nginx is not running'))
    nginxInfo.value = response.running ? response.info : null
    if (response.running)
      updateLastUpdateTime()
  }

  // Check stub_status availability and get initial data
  async function fetchInitialData() {
    try {
      loading.value = true
      stubStatusLoading.value = true
      error.value = ''

      // Get performance data
      const response = await ngx.detail_status()

      applyPerformanceData(response)
    }
    catch (err) {
      console.error('Failed to get Nginx performance data:', err)
      error.value = $gettext('Failed to get performance data')
      nginxInfo.value = null
    }
    finally {
      loading.value = false
      stubStatusLoading.value = false
    }
  }

  return {
    loading,
    nginxInfo,
    error,
    formattedUpdateTime,
    updateLastUpdateTime,
    fetchInitialData,
    applyPerformanceData,
    stubStatusEnabled,
    stubStatusLoading,
    stubStatusError,
  }
}
