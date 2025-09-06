<script setup lang="tsx">
import { CheckOutlined, CopyOutlined, InfoCircleFilled, WarningOutlined } from '@ant-design/icons-vue'
import { UseClipboard } from '@vueuse/components'
import backup from '@/api/backup'

const { message } = useGlobalApp()

const isCreatingBackup = ref(false)
const showSecurityModal = ref(false)
const currentSecurityToken = ref('')
const isCopied = ref(false)

async function handleCreateBackup() {
  try {
    isCreatingBackup.value = true
    const response = await backup.createBackup()

    // Extract filename from Content-Disposition header if available
    const contentDisposition = response.headers['content-disposition']
    let filename = 'nginx-ui-backup.zip'
    if (contentDisposition) {
      const filenameMatch = contentDisposition.match(/filename=(.+)/)
      if (filenameMatch && filenameMatch[1]) {
        filename = filenameMatch[1].replace(/"/g, '')
      }
    }

    // Extract security token from header
    const securityToken = response.headers['x-backup-security']

    // Create download link
    const url = window.URL.createObjectURL(new Blob([response.data]))
    const link = document.createElement('a')
    link.href = url
    link.setAttribute('download', filename)
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)

    // Show security token to user
    if (securityToken) {
      message.success($gettext('Backup has been downloaded successfully'))

      // Show the security token modal
      currentSecurityToken.value = securityToken
      showSecurityModal.value = true
    }
  }
  catch (error) {
    console.error('Backup download failed:', error)
  }
  finally {
    isCreatingBackup.value = false
  }
}

function handleCloseModal() {
  showSecurityModal.value = false
}

function handleCopy(copy) {
  copy()
  isCopied.value = true
  setTimeout(() => {
    isCopied.value = false
  }, 2000)
}
</script>

<template>
  <ACard :title="$gettext('System Backup')" :bordered="false">
    <AAlert
      show-icon
      type="info"
      :message="$gettext('Create system backups including Nginx configuration and Nginx UI settings. Backup files will be automatically downloaded to your computer.')"
      class="mb-4"
    />

    <div class="flex justify-between">
      <ASpace>
        <AButton
          type="primary"
          :loading="isCreatingBackup"
          @click="handleCreateBackup"
        >
          {{ $gettext('Create Backup') }}
        </AButton>
      </ASpace>
    </div>

    <!-- Security Token Modal Component -->
    <AModal
      v-model:open="showSecurityModal"
      :title="$gettext('Security Token Information')"
      :mask-closable="false"
      :centered="true"
      class="backup-token-modal"
      width="550"
      @ok="handleCloseModal"
    >
      <template #icon>
        <InfoCircleFilled style="color: #1677ff; font-size: 22px" />
      </template>

      <div class="security-token-info py-2">
        <p class="mb-4">
          {{ $gettext('Please save this security token, you will need it for restoration:') }}
        </p>

        <div class="token-display mb-5">
          <div class="token-container p-4 bg-gray-50 border border-gray-200 rounded-md mb-2">
            <div class="token-text font-mono select-all break-all leading-relaxed">
              {{ currentSecurityToken }}
            </div>
          </div>

          <div class="flex justify-end mt-3">
            <UseClipboard v-slot="{ copy }" :source="currentSecurityToken">
              <AButton
                type="primary"
                :style="{ backgroundColor: isCopied ? '#52c41a' : undefined }"
                @click="handleCopy(copy)"
              >
                <template #icon>
                  <CheckOutlined v-if="isCopied" />
                  <CopyOutlined v-else />
                </template>
                {{ isCopied ? $gettext('Copied!') : $gettext('Copy') }}
              </AButton>
            </UseClipboard>
          </div>
        </div>

        <div class="warning-box flex items-start bg-red-50 border border-red-200 p-4 rounded-md">
          <WarningOutlined class="text-red-500 mt-0.5 mr-2 flex-shrink-0" />
          <div>
            <p class="text-red-600 font-medium mb-1">
              {{ $gettext('Warning') }}
            </p>
            <p class="text-red-600 mb-0 text-sm leading-relaxed">
              {{ $gettext('This token will only be shown once and cannot be retrieved later. Please make sure to save it in a secure location.') }}
            </p>
          </div>
        </div>
      </div>

      <template #footer>
        <AButton type="primary" @click="handleCloseModal">
          {{ $gettext('OK') }}
        </AButton>
      </template>
    </AModal>
  </ACard>
</template>

<style scoped>
.security-token-info {
  text-align: left;
}
.token-container {
  word-break: break-all;
  box-shadow: inset 0 1px 2px rgba(0, 0, 0, 0.05);
}
.token-text {
  line-height: 1.6;
}

/* Dark mode optimization */
:deep(.backup-token-modal) {
  /* Modal background */
  .ant-modal-content {
    background-color: #1f1f1f;
  }

  /* Modal title */
  .ant-modal-header {
    background-color: #1f1f1f;
    border-bottom: 1px solid #303030;
  }

  .ant-modal-title {
    color: #e6e6e6;
  }

  /* Modal content */
  .ant-modal-body {
    color: #e6e6e6;
  }

  /* Modal footer */
  .ant-modal-footer {
    border-top: 1px solid #303030;
    background-color: #1f1f1f;
  }

  /* Close button */
  .ant-modal-close-x {
    color: #e6e6e6;
  }
}

/* Token container dark mode styles */
.dark {
  .token-container {
    background-color: #262626 !important;
    border-color: #303030 !important;
    box-shadow: inset 0 1px 2px rgba(0, 0, 0, 0.2);
  }

  .token-text {
    color: #d9d9d9;
  }

  /* Warning box dark mode */
  .warning-box {
    background-color: rgba(255, 77, 79, 0.1);
    border-color: rgba(255, 77, 79, 0.3);

    p {
      color: #ff7875;
    }
  }
}

/* Dark mode support via media query */
@media (prefers-color-scheme: dark) {
  .token-container {
    background-color: #262626 !important;
    border-color: #303030 !important;
  }

  .token-text {
    color: #d9d9d9;
  }

  .warning-box {
    background-color: rgba(255, 77, 79, 0.1);
    border-color: rgba(255, 77, 79, 0.3);

    p {
      color: #ff7875;
    }
  }
}
</style>
