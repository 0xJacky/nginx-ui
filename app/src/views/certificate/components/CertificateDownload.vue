<script setup lang="ts">
import type { Cert } from '@/api/cert'
import { DownloadOutlined } from '@ant-design/icons-vue'

interface Props {
  data: Cert
}

const props = defineProps<Props>()

const { message } = App.useApp()

// Download state
const isDownloading = ref(false)

// Check if certificate files can be downloaded
const canDownloadCertificates = computed(() => {
  return !!(props.data.ssl_certificate?.trim() && props.data.ssl_certificate_key?.trim())
})

// Download individual files
function downloadFile(content: string, filename: string, mimeType = 'text/plain') {
  const blob = new Blob([content], { type: mimeType })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  URL.revokeObjectURL(url)
}

// Download certificate files
async function downloadCertificateFiles() {
  if (!canDownloadCertificates.value) {
    message.error($gettext('Certificate content and private key content cannot be empty'))
    return
  }

  if (!props.data.name?.trim()) {
    message.error($gettext('Certificate name cannot be empty'))
    return
  }

  try {
    isDownloading.value = true

    // Validate certificate content format
    const certContent = props.data.ssl_certificate.trim()
    const keyContent = props.data.ssl_certificate_key.trim()

    if (!certContent.includes('-----BEGIN CERTIFICATE-----') && !certContent.includes('-----BEGIN ')) {
      message.error($gettext('Invalid certificate format'))
      return
    }

    if (!keyContent.includes('-----BEGIN') || !keyContent.includes('PRIVATE KEY-----')) {
      message.error($gettext('Invalid private key format'))
      return
    }

    // Download certificate file
    downloadFile(certContent, `${props.data.name}.crt`, 'application/x-x509-ca-cert')

    // Download private key file with a small delay
    setTimeout(() => {
      downloadFile(keyContent, `${props.data.name}.key`, 'application/x-pem-file')
    }, 100)

    message.success($gettext('Certificate files downloaded successfully'))
  }
  catch (error) {
    console.error('Download error:', error)
    message.error($gettext('Failed to download certificate files'))
  }
  finally {
    isDownloading.value = false
  }
}
</script>

<template>
  <div v-if="canDownloadCertificates" class="certificate-download">
    <AButton
      type="primary"
      size="small"
      :loading="isDownloading"
      @click="downloadCertificateFiles"
    >
      <template #icon>
        <DownloadOutlined />
      </template>
      {{ $gettext('Download Certificate Files') }}
    </AButton>
  </div>
</template>

<style scoped lang="less">
.certificate-download {
  margin-bottom: 12px;
}
</style>
