<script setup lang="tsx">
import { SyncOutlined } from '@antdv-next/icons'
import { useGlobalStore, useWebSocketEventBusStore } from '@/pinia'

const websocketEventBus = useWebSocketEventBusStore()
let processingStatusSubscriptionId: string | null = null

const globalStore = useGlobalStore()
const { processingStatus } = storeToRefs(globalStore)

onMounted(() => {
  processingStatusSubscriptionId = websocketEventBus.subscribe('processing_status', data => {
    // A server without plugin entries sends no list.
    processingStatus.value = { ...data, plugins: data?.plugins ?? [] }
  })
})

onUnmounted(() => {
  if (processingStatusSubscriptionId) {
    websocketEventBus.unsubscribe(processingStatusSubscriptionId)
  }
})

const pluginEntries = computed(() => processingStatus.value.plugins ?? [])

const isProcessing = computed(() => {
  const status = processingStatus.value

  return status.index_scanning
    || status.auto_cert_processing
    || status.nginx_log_indexing
    || pluginEntries.value.length > 0
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
          <!-- Plugin labels are English source strings, plugins supply the translations. -->
          <div
            v-for="entry in pluginEntries"
            :key="`${entry.plugin_id}:${entry.key}`"
          >
            <ABadge
              status="processing"
              :text="$gettext(entry.label)"
            />
          </div>
        </div>
      </template>
      <SyncOutlined spin />
    </APopover>
  </div>
</template>
