<script setup lang="ts">
import type { Cert } from '@/api/cert'
import { CopyOutlined, InboxOutlined } from '@ant-design/icons-vue'
import { useClipboard } from '@vueuse/core'
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
function handleCertificateUpload(content: string) {
  data.value.ssl_certificate = content
}

// Handle private key file upload
function handlePrivateKeyUpload(content: string) {
  data.value.ssl_certificate_key = content
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
        handleCertificateUpload(content)
      }
      else {
        handlePrivateKeyUpload(content)
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
        @upload="handleCertificateUpload"
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
        @upload="handlePrivateKeyUpload"
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
      font-weight: 500;
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
</style>
