<script setup lang="ts">
import { UploadOutlined } from '@ant-design/icons-vue'
import { message } from 'ant-design-vue'

interface Props {
  type: 'certificate' | 'key'
  disabled?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  disabled: false,
})

const emit = defineEmits<{
  upload: [content: string]
}>()

// File upload state
const fileInput = ref<HTMLInputElement>()

// Supported file extensions
const certificateExtensions = ['.crt', '.pem', '.cer', '.cert', '.csr']
const keyExtensions = ['.key', '.pem', '.private']

// Get accepted file extensions based on type
const acceptedExtensions = computed(() => {
  return props.type === 'certificate' ? certificateExtensions : keyExtensions
})

// Get accept attribute for input
const acceptAttribute = computed(() => {
  return acceptedExtensions.value.join(',')
})

// File size limit (5MB)
const maxFileSize = 5 * 1024 * 1024

// Validate file type and size
function validateFile(file: File): boolean {
  const fileName = file.name.toLowerCase()
  const isValidExtension = acceptedExtensions.value.some(ext => fileName.endsWith(ext))

  if (!isValidExtension) {
    const typeText = props.type === 'certificate' ? $gettext('certificate') : $gettext('private key')
    message.error($gettext('Please select a valid %{type} file (%{extensions})', {
      type: typeText,
      extensions: acceptedExtensions.value.join(', '),
    }))
    return false
  }

  if (file.size > maxFileSize) {
    message.error($gettext('File size cannot exceed 5MB'))
    return false
  }

  return true
}

// Read file content
function readFileContent(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = e => {
      const content = e.target?.result as string
      resolve(content)
    }
    reader.onerror = () => {
      reject(new Error($gettext('Failed to read file')))
    }
    reader.readAsText(file)
  })
}

// Handle file upload
async function handleFileUpload(file: File) {
  if (!validateFile(file)) {
    return
  }

  try {
    const content = await readFileContent(file)

    // Basic content validation
    if (props.type === 'certificate') {
      if (!content.includes('-----BEGIN CERTIFICATE-----') && !content.includes('-----BEGIN ')) {
        message.error($gettext('Invalid certificate format'))
        return
      }
    }
    else if (props.type === 'key') {
      if (!content.includes('-----BEGIN') || !content.includes('PRIVATE KEY-----')) {
        message.error($gettext('Invalid private key format'))
        return
      }
    }

    emit('upload', content)
    message.success($gettext('File uploaded successfully'))
  }
  catch (error) {
    console.error('File upload error:', error)
    message.error($gettext('Failed to upload file'))
  }
}

// Handle file selection from input
function handleFileSelect(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (file) {
    handleFileUpload(file)
  }
  // Reset input value to allow selecting the same file again
  input.value = ''
}

// Get upload text based on type
const uploadText = computed(() => {
  const typeText = props.type === 'certificate' ? $gettext('certificate') : $gettext('private key')
  return $gettext('Upload %{type} File', { type: typeText })
})
</script>

<template>
  <div class="certificate-file-upload">
    <input
      ref="fileInput"
      type="file"
      :accept="acceptAttribute"
      style="display: none"
      @change="handleFileSelect"
    >
    <AButton
      size="small"
      type="dashed"
      :disabled="disabled"
      @click="fileInput?.click()"
    >
      <template #icon>
        <UploadOutlined />
      </template>
      {{ uploadText }}
    </AButton>
    <span class="ml-2 text-gray-500 text-sm">
      {{ $gettext('or drag file to editor below') }}
    </span>
  </div>
</template>

<style scoped lang="less">
.certificate-file-upload {
  margin-bottom: 12px;
}
</style>
