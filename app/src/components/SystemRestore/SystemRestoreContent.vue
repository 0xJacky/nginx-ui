<script setup lang="ts">
import type { RestoreOptions, RestoreResponse } from '@/api/backup'
import type { UploadFile } from 'ant-design-vue'
import backup from '@/api/backup'
import { InboxOutlined } from '@ant-design/icons-vue'
import { message } from 'ant-design-vue'

// Define props using TypeScript interface
interface SystemRestoreProps {
  showTitle?: boolean
  showNginxOptions?: boolean
}

// Define emits using TypeScript interface
interface SystemRestoreEmits {
  (e: 'restoreSuccess', options: { restoreNginx: boolean, restoreNginxUI: boolean }): void
  (e: 'restoreError', error: Error): void
}

withDefaults(defineProps<SystemRestoreProps>(), {
  showTitle: true,
  showNginxOptions: true,
})

const emit = defineEmits<SystemRestoreEmits>()

// Use UploadFile from ant-design-vue
const uploadFiles = ref<UploadFile[]>([])
const isRestoring = ref(false)

const formModel = reactive({
  securityToken: '',
  restoreNginx: true,
  restoreNginxUI: true,
  verifyHash: true,
})

// Add two variables to control modal display and countdown
const showRestoreModal = ref(false)
const countdown = ref(5)
const countdownTimer = ref<ReturnType<typeof setInterval> | null>(null)

// Reset countdown function
function resetCountdown() {
  countdown.value = 5
  showRestoreModal.value = true

  // Clear any existing timer
  if (countdownTimer.value) {
    clearInterval(countdownTimer.value)
  }

  // Start countdown timer
  countdownTimer.value = setInterval(() => {
    countdown.value--
    if (countdown.value <= 0 && countdownTimer.value) {
      clearInterval(countdownTimer.value)
    }
  }, 1000)
}

// Handle OK button click
function handleModalOk() {
  if (countdownTimer.value) {
    clearInterval(countdownTimer.value)
  }
  // Emit success event with restore options
  emit('restoreSuccess', {
    restoreNginx: formModel.restoreNginx,
    restoreNginxUI: formModel.restoreNginxUI,
  })
}

function handleBeforeUpload(file: File) {
  // Check if file type is zip
  const isZip = file.name.toLowerCase().endsWith('.zip')
  if (!isZip) {
    message.error($gettext('Only zip files are allowed'))
    uploadFiles.value = []
    return
  }

  // Create UploadFile object and directly manage uploadFiles
  const uploadFile = {
    uid: Date.now().toString(),
    name: file.name,
    status: 'done',
    size: file.size,
    type: file.type,
    originFileObj: file,
  } as UploadFile

  // Keep only the current file
  uploadFiles.value = [uploadFile]

  // Prevent default upload behavior
  return false
}

// Handle file removal
function handleRemove() {
  uploadFiles.value = []
}

async function doRestore() {
  if (uploadFiles.value.length === 0) {
    message.warning($gettext('Please select a backup file'))
    return
  }

  if (!formModel.securityToken) {
    message.warning($gettext('Please enter the security token'))
    return
  }

  try {
    isRestoring.value = true

    const uploadedFile = uploadFiles.value[0]
    if (!uploadedFile.originFileObj) {
      message.error($gettext('Invalid file object'))
      return
    }

    const options: RestoreOptions = {
      backup_file: uploadedFile.originFileObj,
      security_token: formModel.securityToken,
      restore_nginx: formModel.restoreNginx,
      restore_nginx_ui: formModel.restoreNginxUI,
      verify_hash: formModel.verifyHash,
    }

    const data = await backup.restoreBackup(options) as RestoreResponse

    message.success($gettext('Restore completed successfully'))

    if (data.nginx_restored) {
      message.info($gettext('Nginx configuration has been restored'))
    }

    if (data.nginx_ui_restored) {
      message.info($gettext('Nginx UI configuration has been restored'))
      // If UI was restored, show the countdown modal
      resetCountdown()
    }
    else {
      // If UI was not restored, emit success event directly
      emit('restoreSuccess', {
        restoreNginx: formModel.restoreNginx,
        restoreNginxUI: formModel.restoreNginxUI,
      })
    }

    if (data.hash_match === false && formModel.verifyHash) {
      message.warning($gettext('Backup file integrity check failed, it may have been tampered with'))
    }

    // Reset form after successful restore
    uploadFiles.value = []
    formModel.securityToken = ''
  }
  catch (error) {
    console.error('Restore failed:', error)
    emit('restoreError', error instanceof Error ? error : new Error(String(error)))
  }
  finally {
    isRestoring.value = false
  }
}
</script>

<template>
  <div>
    <ACard v-if="showTitle" :title="$gettext('System Restore')" :bordered="false">
      <AAlert
        show-icon
        type="warning"
        :message="$gettext('Warning: Restore operation will overwrite current configurations. Make sure you have a valid backup file and security token, and carefully select what to restore.')"
        class="mb-4"
      />

      <AUploadDragger
        :file-list="uploadFiles"
        :multiple="false"
        :max-count="1"
        accept=".zip"
        :before-upload="handleBeforeUpload"
        @remove="handleRemove"
      >
        <p class="ant-upload-drag-icon">
          <InboxOutlined />
        </p>
        <p class="ant-upload-text">
          {{ $gettext('Click or drag backup file to this area to upload') }}
        </p>
        <p class="ant-upload-hint">
          {{ $gettext('Supported file type: .zip') }}
        </p>
      </AUploadDragger>

      <AForm
        v-if="uploadFiles.length > 0"
        :model="formModel"
        layout="vertical"
        class="mt-4"
      >
        <AFormItem :label="$gettext('Security Token')">
          <AInput
            v-model:value="formModel.securityToken"
            :placeholder="$gettext('Please enter the security token received during backup')"
          />
        </AFormItem>

        <AFormItem>
          <ACheckbox v-model:checked="formModel.verifyHash" :disabled="true">
            {{ $gettext('Verify Backup File Integrity') }}
          </ACheckbox>
        </AFormItem>

        <template v-if="showNginxOptions">
          <AFormItem>
            <ACheckbox v-model:checked="formModel.restoreNginx">
              {{ $gettext('Restore Nginx Configuration') }}
            </ACheckbox>
            <div class="text-gray-500 ml-6 mt-1 text-sm">
              <p class="mb-0">
                {{ $gettext('This will restore all Nginx configuration files. Nginx will restart after the restoration is complete.') }}
              </p>
            </div>
          </AFormItem>

          <AFormItem>
            <ACheckbox v-model:checked="formModel.restoreNginxUI">
              {{ $gettext('Restore Nginx UI Configuration') }}
            </ACheckbox>
            <div class="text-gray-500 ml-6 mt-1 text-sm">
              <p class="mb-0">
                {{ $gettext('This will restore configuration files and database. Nginx UI will restart after the restoration is complete.') }}
              </p>
            </div>
          </AFormItem>
        </template>

        <AFormItem>
          <AButton type="primary" :loading="isRestoring" @click="doRestore">
            {{ $gettext('Start Restore') }}
          </AButton>
        </AFormItem>
      </AForm>
    </ACard>
    <div v-else>
      <AAlert
        show-icon
        type="warning"
        :message="$gettext('Warning: Restore operation will overwrite current configurations. Make sure you have a valid backup file and security token, and carefully select what to restore.')"
        class="mb-4"
      />

      <AUploadDragger
        :file-list="uploadFiles"
        :multiple="false"
        :max-count="1"
        accept=".zip"
        :before-upload="handleBeforeUpload"
        @remove="handleRemove"
      >
        <p class="ant-upload-drag-icon">
          <InboxOutlined />
        </p>
        <p class="ant-upload-text">
          {{ $gettext('Click or drag backup file to this area to upload') }}
        </p>
        <p class="ant-upload-hint">
          {{ $gettext('Supported file type: .zip') }}
        </p>
      </AUploadDragger>

      <AForm
        v-if="uploadFiles.length > 0"
        :model="formModel"
        layout="vertical"
        class="mt-4"
      >
        <AFormItem :label="$gettext('Security Token')">
          <AInput
            v-model:value="formModel.securityToken"
            :placeholder="$gettext('Please enter the security token received during backup')"
          />
        </AFormItem>

        <AFormItem>
          <ACheckbox v-model:checked="formModel.verifyHash" :disabled="true">
            {{ $gettext('Verify Backup File Integrity') }}
          </ACheckbox>
        </AFormItem>

        <template v-if="showNginxOptions">
          <AFormItem>
            <ACheckbox v-model:checked="formModel.restoreNginx">
              {{ $gettext('Restore Nginx Configuration') }}
            </ACheckbox>
            <div class="text-gray-500 ml-6 mt-1 text-sm">
              <p class="mb-0">
                {{ $gettext('This will restore all Nginx configuration files. Nginx will restart after the restoration is complete.') }}
              </p>
            </div>
          </AFormItem>

          <AFormItem>
            <ACheckbox v-model:checked="formModel.restoreNginxUI">
              {{ $gettext('Restore Nginx UI Configuration') }}
            </ACheckbox>
            <div class="text-gray-500 ml-6 mt-1 text-sm">
              <p class="mb-0">
                {{ $gettext('This will restore configuration files and database. Nginx UI will restart after the restoration is complete.') }}
              </p>
            </div>
          </AFormItem>
        </template>

        <AFormItem>
          <AButton type="primary" :loading="isRestoring" @click="doRestore">
            {{ $gettext('Start Restore') }}
          </AButton>
        </AFormItem>
      </AForm>
    </div>

    <!-- Modal for countdown -->
    <AModal
      v-model:open="showRestoreModal"
      :title="$gettext('Automatic Restart')"
      :mask-closable="false"
    >
      <p>
        {{ $gettext('Nginx UI configuration has been restored and will restart automatically in a few seconds.') }}
      </p>
      <p v-if="countdown > 0">
        {{ $gettext('You can close this dialog in %{countdown} seconds', { countdown: countdown.toString() }) }}
      </p>
      <p v-else>
        {{ $gettext('You can close this dialog now') }}
      </p>
      <template #footer>
        <AButton
          type="primary"
          :disabled="countdown > 0"
          @click="handleModalOk"
        >
          {{ countdown > 0 ? `OK (${countdown}s)` : 'OK' }}
        </AButton>
      </template>
    </AModal>
  </div>
</template>
