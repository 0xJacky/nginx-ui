<script setup lang="ts">
import certApi from '@/api/cert'
import type { Cert } from '@/api/cert'
import { DownloadOutlined } from '@antdv-next/icons'

interface Props {
  data: Cert
  inline?: boolean
}

const props = defineProps<Props>()

const { message } = App.useApp()

// Download state
const isDownloading = ref(false)
const modalVisible = ref(false)
const selectedFormats = ref<Array<'crt' | 'key' | 'pfx'>>(['crt', 'key'])
const pfxPassword = ref('')

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

function downloadBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  URL.revokeObjectURL(url)
}

function openDownloadModal() {
  selectedFormats.value = ['crt', 'key']
  pfxPassword.value = ''
  modalVisible.value = true
}

const requiresPfxPasswordInput = computed(() => selectedFormats.value.includes('pfx'))

// Download selected certificate files
async function downloadCertificateFiles() {
  if (!canDownloadCertificates.value) {
    message.error($gettext('Certificate content and private key content cannot be empty'))
    return
  }

  if (selectedFormats.value.length === 0) {
    message.error($gettext('Please select at least one download format'))
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

    if (selectedFormats.value.includes('crt'))
      downloadFile(certContent, `${props.data.name}.crt`, 'application/x-x509-ca-cert')

    if (selectedFormats.value.includes('key'))
      downloadFile(keyContent, `${props.data.name}.key`, 'application/x-pem-file')

    if (selectedFormats.value.includes('pfx')) {
      if (!props.data.id) {
        message.error($gettext('Certificate ID cannot be empty'))
        return
      }

      const pfxBlob = await certApi.download_file(props.data.id, {
        format: 'pfx',
        pfx_password: pfxPassword.value,
      })
      downloadBlob(pfxBlob, `${props.data.name}.pfx`)
    }

    modalVisible.value = false
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
  <div v-if="canDownloadCertificates" :class="['certificate-download', { 'is-inline': inline }]">
    <AButton
      type="primary"
      ghost
      size="middle"
      @click="openDownloadModal"
    >
      <template #icon>
        <DownloadOutlined />
      </template>
      {{ $gettext('Download Certificate Files') }}
    </AButton>

    <AModal
      v-model:open="modalVisible"
      :title="$gettext('Download Certificate Files')"
      :confirm-loading="isDownloading"
      @ok="downloadCertificateFiles"
    >
      <AForm layout="vertical">
        <AFormItem :label="$gettext('Download Format')">
          <ACheckboxGroup v-model:value="selectedFormats">
            <div class="flex flex-col gap-2">
              <ACheckbox value="crt">
                {{ $gettext('Certificate (.crt)') }}
              </ACheckbox>
              <ACheckbox value="key">
                {{ $gettext('Private Key (.key)') }}
              </ACheckbox>
              <ACheckbox value="pfx">
                {{ $gettext('PFX (.pfx, CRT + KEY for Windows import)') }}
              </ACheckbox>
            </div>
          </ACheckboxGroup>
        </AFormItem>

        <AFormItem
          v-if="requiresPfxPasswordInput"
          :label="$gettext('PFX Password')"
          :extra="$gettext('Leave empty if you do not want to set a password')"
        >
          <AInputPassword
            v-model:value="pfxPassword"
            :placeholder="$gettext('Enter PFX password')"
          />
        </AFormItem>
      </AForm>
    </AModal>
  </div>
</template>

<style scoped lang="less">
.certificate-download {
  margin-bottom: 12px;
}

.certificate-download.is-inline {
  margin-bottom: 0;
}
</style>
