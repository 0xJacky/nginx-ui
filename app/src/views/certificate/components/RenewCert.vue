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
    <AButton
      type="default"
      class="mb-6 renew-warning-btn"
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

<style scoped lang="less">
.renew-warning-btn {
  color: #ffd666;
  border-color: #ffd666;

  &:hover,
  &:focus {
    color: #ffc53d;
    border-color: #ffc53d;
  }

  &:active {
    color: #faad14;
    border-color: #faad14;
  }
}

.renew-warning-btn.ant-btn[disabled],
.renew-warning-btn.ant-btn-disabled {
  color: rgb(0 0 0 / 40%);
  border-color: #ffe7ba;
}
</style>
