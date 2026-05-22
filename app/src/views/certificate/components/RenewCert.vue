<script setup lang="ts">
import type { AutoCertOptions } from '@/api/auto_cert'
import { useGlobalStore } from '@/pinia'
import { useCertStore } from '../store'
import IssueCertModal from './IssueCertModal.vue'

defineProps<{
  options: AutoCertOptions
}>()

const emit = defineEmits<{
  renewed: [void]
}>()

const { message } = App.useApp()
const certStore = useCertStore()
const refModal = useTemplateRef('refModal')

async function issueCert() {
  await certStore.save()
  message.success($gettext('Save successfully'))

  refModal.value?.start().then(() => {
    message.success($gettext('Renew successfully'))
    emit('renewed')
  })
}

const globalStore = useGlobalStore()
const { processingStatus } = storeToRefs(globalStore)
</script>

<template>
  <div>
    <AButton
      type="primary"
      ghost
      class="mb-6"
      :disabled="processingStatus.auto_cert_processing"
      @click="issueCert"
    >
      {{ $gettext('Renew Certificate') }}
    </AButton>
    <span v-if="processingStatus.auto_cert_processing" class="ml-4">
      {{ $gettext('AutoCert is running, please wait...') }}
    </span>
    <IssueCertModal
      ref="refModal"
      :title="$gettext('Renew Certificate')"
      :options
    />
  </div>
</template>
