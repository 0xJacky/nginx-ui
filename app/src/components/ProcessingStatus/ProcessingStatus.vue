<script setup lang="tsx">
import { SyncOutlined } from '@ant-design/icons-vue'
import { useGlobalStore, useWebSocketEventBusStore } from '@/pinia'

const websocketEventBus = useWebSocketEventBusStore()
let processingStatusSubscriptionId: string | null = null

const globalStore = useGlobalStore()
const { processingStatus } = storeToRefs(globalStore)

onMounted(() => {
  processingStatusSubscriptionId = websocketEventBus.subscribe('processing_status', data => {
    processingStatus.value = data
  })
})

onUnmounted(() => {
  if (processingStatusSubscriptionId) {
    websocketEventBus.unsubscribe(processingStatusSubscriptionId)
  }
})

const isProcessing = computed(() => {
  return Object.values(processingStatus.value).some(v => v)
})
</script>

<template>
  <div v-if="isProcessing">
    <APopover>
      <template #content>
        <div>
          <div>
            <ABadge
              v-if="processingStatus.index_scanning"
              status="processing"
              :text="$gettext('Indexing...')"
            />
          </div>
          <div>
            <ABadge
              v-if="processingStatus.auto_cert_processing"
              status="processing"
              :text="$gettext('AutoCert is running...')"
            />
          </div>
          <div>
            <ABadge
              v-if="processingStatus.nginx_log_indexing"
              status="processing"
              :text="$gettext('Nginx Log Indexing...')"
            />
          </div>
        </div>
      </template>
      <SyncOutlined spin />
    </APopover>
  </div>
</template>
