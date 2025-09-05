<script setup lang="ts">
import type { Cert } from '@/api/cert'
import cert from '@/api/cert'
import { AutoCertState } from '@/constants'
import websocket from '@/lib/websocket'

const props = defineProps<{
  id: number
  disabled?: boolean
  certificate?: Cert // Certificate full information
}>()

const emit = defineEmits(['removed'])

const { message } = App.useApp()

const modalVisible = ref(false)
const confirmLoading = ref(false)
const shouldRevoke = ref(false)
const revokeInput = ref('')

// Check if it's a managed certificate (auto_cert === AutoCertState.Enable)
const isManagedCertificate = computed(() => {
  return props.certificate?.auto_cert === AutoCertState.Enable
})

// Handle certificate deletion
function handleDelete() {
  // Open the combined modal directly
  modalVisible.value = true
}

// Handle confirmation
function handleConfirm() {
  // If revocation is checked but confirmation text is not correct
  if (shouldRevoke.value && revokeInput.value !== $gettext('Revoke')) {
    message.error($gettext('Please type "Revoke" to confirm'))
    return
  }

  confirmLoading.value = true

  if (shouldRevoke.value) {
    // Revoke certificate using WebSocket
    const ws = websocket(`/api/certs/${props.id}/revoke`, false)

    ws.onmessage = m => {
      const response = JSON.parse(m.data)

      if (response.status === 'success') {
        message.success($gettext('Certificate removed successfully'))
        // Close modal and refresh list
        modalVisible.value = false
        confirmLoading.value = false
        emit('removed')
      }
      else if (response.status === 'error') {
        message.error(response.message || $gettext('Failed to revoke certificate'))
        confirmLoading.value = false
      }
    }

    ws.onerror = () => {
      message.error($gettext('WebSocket connection error'))
      confirmLoading.value = false
    }
  }
  else {
    // Only remove certificate from database
    cert.deleteItem(props.id).then(() => {
      message.success($gettext('Certificate removed successfully'))
      modalVisible.value = false
      confirmLoading.value = false
      emit('removed')
    }).catch(error => {
      message.error(error.message || $gettext('Failed to delete certificate'))
      confirmLoading.value = false
    })
  }
}

// Handle modal cancel
function handleCancel() {
  modalVisible.value = false
  shouldRevoke.value = false
  revokeInput.value = ''
}
</script>

<template>
  <div class="inline-block">
    <AButton
      type="link"
      size="small"
      danger
      :disabled
      @click="handleDelete"
    >
      {{ $gettext('Delete') }}
    </AButton>

    <AModal
      v-model:open="modalVisible"
      :title="$gettext('Delete Certificate')"
      :confirm-loading="confirmLoading"
      :ok-button-props="{
        disabled: (shouldRevoke && revokeInput !== $gettext('Revoke')),
      }"
      @ok="handleConfirm"
      @cancel="handleCancel"
    >
      <AAlert
        type="warning"
        show-icon
        :message="$gettext('This operation will only remove the certificate from the database. The certificate files on the file system will not be deleted.')"
        class="mb-4"
      />

      <div v-if="isManagedCertificate" class="mb-4">
        <ACheckbox v-model:checked="shouldRevoke">
          {{ $gettext('Revoke this certificate') }}
        </ACheckbox>
      </div>

      <div v-if="shouldRevoke">
        <AAlert
          type="error"
          show-icon
          :message="$gettext('Revoking a certificate will affect any services currently using it. This action cannot be undone.')"
          class="mb-4"
        />

        <p>{{ $gettext('To confirm revocation, please type "Revoke" in the field below:') }}</p>
        <AInput v-model:value="revokeInput" />
      </div>
    </AModal>
  </div>
</template>
