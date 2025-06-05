<script setup lang="ts">
import type { AutoCertOptions } from '@/api/auto_cert'
import { message } from 'ant-design-vue'
import { useGlobalStore } from '@/pinia'
import ObtainCertLive from '@/views/site/site_edit/components/Cert/ObtainCertLive.vue'
import { useCertStore } from '../store'

const props = defineProps<{
  options: AutoCertOptions
}>()

const emit = defineEmits<{
  renewed: [void]
}>()

const certStore = useCertStore()

const modalVisible = ref(false)
const modalClosable = ref(true)
const refObtainCertLive = useTemplateRef('refObtainCertLive')

async function issueCert() {
  await certStore.save()

  modalVisible.value = true

  const { name, domains, key_type } = props.options

  refObtainCertLive.value?.issue_cert(name!, domains, key_type).then(() => {
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
    <AModal
      v-model:open="modalVisible"
      :title="$gettext('Renew Certificate')"
      :mask-closable="modalClosable"
      :footer="null"
      :closable="modalClosable"
      :width="600"
      force-render
    >
      <ObtainCertLive
        ref="refObtainCertLive"
        v-model:modal-closable="modalClosable"
        v-model:modal-visible="modalVisible"
        :options
      />
    </AModal>
  </div>
</template>

<style lang="less" scoped>
.control-btn {
  display: flex;
  justify-content: flex-end;
}
</style>
