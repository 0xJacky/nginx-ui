<script setup lang="tsx">
import { SyncOutlined } from '@ant-design/icons-vue'
import { useSSE } from '@/composables/useSSE'
import { useGlobalStore } from '@/pinia'

const { connect } = useSSE()

const globalStore = useGlobalStore()
const { processingStatus } = storeToRefs(globalStore)

onMounted(() => {
  connect({
    url: '/api/system/processing',
    onMessage: data => {
      processingStatus.value = data
    },
  })
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
        </div>
      </template>
      <SyncOutlined spin />
    </APopover>
  </div>
</template>
