<script setup lang="ts">
import type { Ref } from 'vue'
import { CheckCircleOutlined, DownloadOutlined, InfoCircleOutlined } from '@ant-design/icons-vue'
import geolite from '@/api/geolite'
import { formatDateTime } from '@/lib/helper'
import websocket from '@/lib/websocket'

interface Emits {
  (e: 'downloadComplete'): void
}

const emit = defineEmits<Emits>()

// GeoLite database state
const geoLiteStatus = ref({
  exists: false,
  path: '',
  size: 0,
  last_modified: '',
})
const geoLiteLoading = ref(false)
const downloading = ref(false)
const downloadProgress = ref(0)
const downloadStatus = ref('active') as Ref<'normal' | 'active' | 'success' | 'exception'>
const downloadMessage = ref('')

const progressStrokeColor = {
  from: '#108ee9',
  to: '#87d068',
}

const downloadProgressComputed = computed(() => {
  return Number.parseFloat(downloadProgress.value.toFixed(1))
})

// Check GeoLite database status
async function checkGeoLiteStatus() {
  try {
    geoLiteLoading.value = true
    const status = await geolite.getStatus()
    geoLiteStatus.value = status
  }
  catch (e) {
    console.error('Failed to check GeoLite status:', e)
  }
  finally {
    geoLiteLoading.value = false
  }
}

// Download GeoLite database
function downloadGeoLiteDB() {
  downloading.value = true
  downloadStatus.value = 'active'
  downloadProgress.value = 0
  downloadMessage.value = $gettext('Starting download...')

  const ws = websocket('/api/geolite/download', false)

  let isFailed = false
  let currentPhase = 'download' // 'download' or 'decompress'

  ws.onopen = () => {
    // WebSocket connected, server will start download
  }

  ws.onmessage = async m => {
    const r = JSON.parse(m.data)

    // Update message and detect phase changes
    if (r.message) {
      downloadMessage.value = r.message

      // Detect phase transition
      if (r.message.toLowerCase().includes('decompress')) {
        currentPhase = 'decompress'
      }
    }

    switch (r.status) {
      case 'info':
        // Info messages handled above
        break
      case 'progress': {
        // Map progress to correct range based on phase
        const actualProgress = currentPhase === 'download'
          ? (r.progress / 100) * 50 // Download phase: 0-50%
          : 50 + (r.progress / 100) * 50 // Decompress phase: 50-100%

        downloadProgress.value = Math.min(actualProgress, 100)
        break
      }
      case 'error':
        downloadStatus.value = 'exception'
        isFailed = true
        break
      default:
        break
    }
  }

  ws.onerror = () => {
    isFailed = true
    downloadStatus.value = 'exception'
    downloadMessage.value = $gettext('Download failed')
  }

  ws.onclose = async () => {
    if (isFailed) {
      downloading.value = false
      return
    }

    downloadStatus.value = 'success'
    downloadProgress.value = 100
    downloadMessage.value = $gettext('Download complete')

    // Refresh status
    await checkGeoLiteStatus()

    // Emit completion event
    emit('downloadComplete')

    // Reset after 2 seconds
    setTimeout(() => {
      downloading.value = false
      downloadProgress.value = 0
      downloadMessage.value = ''
    }, 2000)
  }
}

// Auto-check status on mount
onMounted(() => {
  checkGeoLiteStatus()
})

// Expose methods for parent components
defineExpose({
  checkGeoLiteStatus,
  downloadGeoLiteDB,
})
</script>

<template>
  <div>
    <AAlert
      v-if="!geoLiteStatus.exists && !downloading"
      :message="$gettext('GeoLite2 Database Required')"
      type="info"
      show-icon
      :icon="h(InfoCircleOutlined)"
      class="mb-3"
    >
      <template #description>
        <div class="space-y-2">
          <p>{{ $gettext('The GeoLite2 database is required for offline geographic IP analysis. Please download it to enable this feature.') }}</p>
          <p class="text-sm">
            {{ $gettext('Alternatively, if you cannot download the database, you can manually place GeoLite2-City.mmdb in the same directory as app.ini.') }}
          </p>
        </div>
      </template>
    </AAlert>

    <AAlert
      v-else-if="geoLiteStatus.exists && !downloading"
      :message="$gettext('GeoLite2 Database Installed')"
      type="success"
      show-icon
      :icon="h(CheckCircleOutlined)"
      class="mb-3"
      banner
    />

    <div class="space-y-3">
      <!-- Download Button -->
      <div class="flex items-center space-x-3">
        <AButton
          v-if="!geoLiteStatus.exists"
          type="primary"
          :loading="geoLiteLoading"
          :disabled="downloading"
          @click="downloadGeoLiteDB"
        >
          <DownloadOutlined />
          {{ $gettext('Download GeoLite2 Database') }}
        </AButton>
        <AButton
          v-else
          :loading="geoLiteLoading"
          :disabled="downloading"
          @click="downloadGeoLiteDB"
        >
          <DownloadOutlined />
          {{ $gettext('Re-download Database') }}
        </AButton>
        <ATypographyText v-if="geoLiteStatus.exists && !downloading" type="secondary" class="text-xs">
          {{ $gettext('Last updated:') }} {{ formatDateTime(geoLiteStatus.last_modified) }}
        </ATypographyText>
      </div>

      <!-- Inline Progress Bar -->
      <div v-if="downloading" class="download-progress-section">
        <AProgress
          :stroke-color="progressStrokeColor"
          :percent="downloadProgressComputed"
          :status="downloadStatus"
        />
      </div>
    </div>
  </div>
</template>

<style scoped lang="less">

</style>
