<script setup lang="ts">
import type { Cert } from '@/api/cert'
import { CopyOutlined, InboxOutlined } from '@ant-design/icons-vue'
import { useClipboard } from '@vueuse/core'
import config from '@/api/config'
import CodeEditor from '@/components/CodeEditor'
import CertificateFileUpload from './CertificateFileUpload.vue'

interface Props {
  data: Cert
  errors?: Record<string, string>
  readonly: boolean
}

defineProps<Props>()

const { message } = App.useApp()

// Use defineModel for two-way binding
const data = defineModel<Cert>('data', { required: true })

const { copy } = useClipboard()

// Lazy load nginx config base path
const nginxBasePath = ref<string>('')
const basePathLoaded = ref(false)

async function loadNginxBasePath() {
  if (basePathLoaded.value)
    return

  try {
    const res = await config.get_base_path()
    nginxBasePath.value = res.base_path
    basePathLoaded.value = true
  }
  catch (error) {
    console.error('Failed to load nginx base path:', error)
    message.warning($gettext('Failed to load nginx configuration path'))
  }
}

// Generate slug from name or filename
function generateSlug(text: string): string {
  let result = text
    .toLowerCase()
    .replace(/\s+/g, '_') // Replace spaces with underscores
    .replace(/[^a-z0-9_./-]/g, '') // Remove invalid characters

  // Remove leading dots
  while (result.startsWith('.')) {
    result = result.slice(1)
  }

  // Remove trailing dots
  while (result.endsWith('.')) {
    result = result.slice(0, -1)
  }

  return result
}

// Auto-generate certificate paths and name
async function autoGeneratePaths(fileName: string, _type: 'certificate' | 'key') {
  // Only generate paths in add mode
  if (data.value.id)
    return

  await loadNginxBasePath()

  if (!nginxBasePath.value)
    return

  // Extract base name from filename (remove extension)
  const baseName = fileName.replace(/\.(crt|pem|cer|cert|key|private)$/i, '')

  // Auto-fill name if empty
  if (!data.value.name) {
    data.value.name = baseName
  }

  // Generate directory name from cert name or filename
  const slug = data.value.name
    ? generateSlug(data.value.name)
    : generateSlug(baseName)

  if (!slug)
    return

  const certDir = `${nginxBasePath.value}/ssl/${slug}`

  // Auto-fill certificate path if empty
  if (!data.value.ssl_certificate_path) {
    data.value.ssl_certificate_path = `${certDir}/fullchain.cer`
  }

  // Auto-fill key path if empty
  if (!data.value.ssl_certificate_key_path) {
    data.value.ssl_certificate_key_path = `${certDir}/private.key`
  }
}

async function copyToClipboard(text: string, label: string) {
  if (!text) {
    message.warning($gettext('Nothing to copy'))
    return
  }
  try {
    await copy(text)
    message.success($gettext(`{label} copied to clipboard`).replace('{label}', label))
  }
  catch (error) {
    console.error(error)
    message.error($gettext('Failed to copy to clipboard'))
  }
}

// Drag and drop state
const isDragOverCert = ref(false)
const isDragOverKey = ref(false)

// Handle certificate file upload
function handleCertificateUpload(content: string, fileName?: string) {
  data.value.ssl_certificate = content

  // Auto-generate paths if in add mode
  if (fileName) {
    autoGeneratePaths(fileName, 'certificate')
  }
}

// Handle private key file upload
function handlePrivateKeyUpload(content: string, fileName?: string) {
  data.value.ssl_certificate_key = content

  // Auto-generate paths if in add mode
  if (fileName) {
    autoGeneratePaths(fileName, 'key')
  }
}

// Drag and drop handlers
function handleDragEnter(e: DragEvent, type: 'certificate' | 'key') {
  e.preventDefault()
  if (type === 'certificate') {
    isDragOverCert.value = true
  }
  else {
    isDragOverKey.value = true
  }
}

function handleDragOver(e: DragEvent) {
  e.preventDefault()
}

function handleDragLeave(e: DragEvent, type: 'certificate' | 'key') {
  e.preventDefault()
  // Only set to false if leaving the component entirely
  const currentTarget = e.currentTarget as HTMLElement
  const relatedTarget = e.relatedTarget as Node
  if (!currentTarget?.contains(relatedTarget)) {
    if (type === 'certificate') {
      isDragOverCert.value = false
    }
    else {
      isDragOverKey.value = false
    }
  }
}

function handleDrop(e: DragEvent, type: 'certificate' | 'key') {
  e.preventDefault()
  if (type === 'certificate') {
    isDragOverCert.value = false
  }
  else {
    isDragOverKey.value = false
  }

  const files = Array.from(e.dataTransfer?.files || [])
  if (files.length > 0) {
    const file = files[0]
    const reader = new FileReader()
    reader.onload = e => {
      const content = e.target?.result as string
      if (type === 'certificate') {
        handleCertificateUpload(content, file.name)
      }
      else {
        handlePrivateKeyUpload(content, file.name)
      }
    }
    reader.readAsText(file)
  }
}
</script>

<template>
  <div class="certificate-content-editor">
    <!-- SSL Certificate Content -->
    <AFormItem
      :validate-status="errors?.ssl_certificate ? 'error' : ''"
      :help="errors?.ssl_certificate === 'certificate'
        ? $gettext('The input is not a SSL Certificate') : ''"
    >
      <template #label>
        <div class="label-with-copy">
          <span class="label-text">{{ $gettext('SSL Certificate Content') }}</span>
          <AButton
            v-if="data.ssl_certificate"
            type="text"
            size="small"
            @click="copyToClipboard(data.ssl_certificate, $gettext('SSL Certificate Content'))"
          >
            <CopyOutlined />
          </AButton>
        </div>
      </template>
      <!-- Certificate File Upload -->
      <CertificateFileUpload
        v-if="!readonly"
        type="certificate"
        @upload="(content, fileName) => handleCertificateUpload(content, fileName)"
      />

      <div
        v-if="!readonly"
        class="code-editor-container"
        @dragenter.prevent="(e) => handleDragEnter(e, 'certificate')"
        @dragover.prevent="handleDragOver"
        @dragleave.prevent="(e) => handleDragLeave(e, 'certificate')"
        @drop.prevent="(e) => handleDrop(e, 'certificate')"
      >
        <CodeEditor
          v-model:content="data.ssl_certificate"
          default-height="300px"
          :readonly="readonly"
          disable-code-completion
          :placeholder="$gettext('Leave blank will not change anything')"
        />
        <div
          v-if="isDragOverCert"
          class="drag-overlay"
        >
          <div class="drag-content">
            <InboxOutlined class="drag-icon" />
            <p>{{ $gettext('Drop certificate file here') }}</p>
          </div>
        </div>
      </div>
      <CodeEditor
        v-else
        v-model:content="data.ssl_certificate"
        default-height="300px"
        :readonly="readonly"
        disable-code-completion
        :placeholder="$gettext('Leave blank will not change anything')"
      />
    </AFormItem>

    <!-- SSL Certificate Key Content -->
    <AFormItem
      :validate-status="errors?.ssl_certificate_key ? 'error' : ''"
      :help="errors?.ssl_certificate_key === 'privatekey'
        ? $gettext('The input is not a SSL Certificate Key') : ''"
    >
      <template #label>
        <div class="label-with-copy">
          <span class="label-text">{{ $gettext('SSL Certificate Key Content') }}</span>
          <AButton
            v-if="data.ssl_certificate_key"
            type="text"
            size="small"
            @click="copyToClipboard(data.ssl_certificate_key, $gettext('SSL Certificate Key Content'))"
          >
            <CopyOutlined />
          </AButton>
        </div>
      </template>
      <!-- Private Key File Upload -->
      <CertificateFileUpload
        v-if="!readonly"
        type="key"
        @upload="(content, fileName) => handlePrivateKeyUpload(content, fileName)"
      />

      <div
        v-if="!readonly"
        class="code-editor-container"
        @dragenter.prevent="(e) => handleDragEnter(e, 'key')"
        @dragover.prevent="handleDragOver"
        @dragleave.prevent="(e) => handleDragLeave(e, 'key')"
        @drop.prevent="(e) => handleDrop(e, 'key')"
      >
        <CodeEditor
          v-model:content="data.ssl_certificate_key"
          default-height="300px"
          :readonly="readonly"
          disable-code-completion
          :placeholder="$gettext('Leave blank will not change anything')"
        />
        <div
          v-if="isDragOverKey"
          class="drag-overlay"
        >
          <div class="drag-content">
            <InboxOutlined class="drag-icon" />
            <p>{{ $gettext('Drop private key file here') }}</p>
          </div>
        </div>
      </div>
      <CodeEditor
        v-else
        v-model:content="data.ssl_certificate_key"
        default-height="300px"
        :readonly="readonly"
        disable-code-completion
        :placeholder="$gettext('Leave blank will not change anything')"
      />
    </AFormItem>
  </div>
</template>

<style scoped lang="less">
.certificate-content-editor {
  .label-with-copy {
    display: flex;
    align-items: center;
    justify-content: space-between;
    width: 100%;

    .label-text {
      color: rgba(0, 0, 0, 0.85);
    }
  }

  .code-editor-container {
    position: relative;

    .drag-overlay {
      position: absolute;
      top: 0;
      left: 0;
      right: 0;
      bottom: 0;
      background-color: rgba(24, 144, 255, 0.1);
      border: 2px dashed #1890ff;
      border-radius: 6px;
      display: flex;
      align-items: center;
      justify-content: center;
      z-index: 10;

      .drag-content {
        text-align: center;
        color: #1890ff;

        .drag-icon {
          font-size: 48px;
          margin-bottom: 16px;
          display: block;
        }

        p {
          font-size: 16px;
          margin: 0;
          font-weight: 500;
        }
      }
    }
  }
}

// 暗夜模式适配
.dark {
  .certificate-content-editor {
    .label-with-copy {
      .label-text {
        color: rgba(255, 255, 255, 0.85);
      }
    }

    .code-editor-container {
      .drag-overlay {
        background-color: rgba(64, 169, 255, 0.15);
        border-color: #177ddc;

        .drag-content {
          color: #40a9ff;

          p {
            color: #40a9ff;
          }
        }
      }
    }
  }
}
</style>
