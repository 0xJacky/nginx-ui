<script setup lang="ts">
import type { AutoCertOptions } from '@/api/auto_cert'
import { SyncOutlined } from '@antdv-next/icons'
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

  // refModal is mounted alongside this button via force-render, so it
  // is guaranteed to be available by the time @click fires.
  refModal.value!.start().then(() => {
    message.success($gettext('Renew successfully'))
    emit('renewed')
  })
}

const globalStore = useGlobalStore()
const { processingStatus } = storeToRefs(globalStore)
</script>

<template>
  <div>
    <ATooltip
      :title="processingStatus.auto_cert_processing ? $gettext('AutoCert is running, please wait...') : undefined"
    >
      <AButton
        color="orange"
        variant="link"
        size="small"
        :loading="processingStatus.auto_cert_processing"
        :aria-label="$gettext('Renew Certificate')"
        @click="issueCert"
      >
        <template #icon>
          <SyncOutlined />
        </template>
        <span class="max-sm:hidden">{{ $gettext('Renew') }}</span>
      </AButton>
    </ATooltip>
    <IssueCertModal
      ref="refModal"
      :title="$gettext('Renew Certificate')"
      :options
    />
  </div>
</template>
