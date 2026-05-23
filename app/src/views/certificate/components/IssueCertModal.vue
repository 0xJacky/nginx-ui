<script setup lang="ts">
import type { AutoCertOptions } from '@/api/auto_cert'
import type { CertificateResult } from '@/api/cert'
import ObtainCertLive from '@/views/site/site_edit/components/Cert/ObtainCertLive.vue'

const props = defineProps<{
  title: string
  options: AutoCertOptions
}>()

const modalVisible = ref(false)
const modalClosable = ref(true)
const refObtainCertLive = useTemplateRef('refObtainCertLive')

function start(): Promise<CertificateResult> {
  modalVisible.value = true
  return new Promise<CertificateResult>((resolve, reject) => {
    nextTick(() => {
      const live = refObtainCertLive.value
      if (!live) {
        reject(new Error('ObtainCertLive not mounted'))
        return
      }
      const { name, domains, key_type } = props.options
      live.issue_cert(name!, domains, key_type).then(resolve).catch(reject)
    })
  })
}

defineExpose({ start })
</script>

<template>
  <AModal
    v-model:open="modalVisible"
    :title="title"
    :mask-closable="modalClosable"
    :closable="modalClosable"
    :footer="null"
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
</template>
