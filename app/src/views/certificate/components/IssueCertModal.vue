<script setup lang="ts">
import type { AutoCertOptions } from '@/api/auto_cert'
import type { CertificateResult } from '@/api/cert'
import ObtainCertLive from '@/views/site/site_edit/components/Cert/ObtainCertLive.vue'
import { createRetryableIssue } from './retryableIssue'

const props = defineProps<{
  title: string
  options: AutoCertOptions
}>()

const modalVisible = ref(false)
const modalClosable = ref(true)
const refObtainCertLive = useTemplateRef('refObtainCertLive')

// A failed attempt keeps the promise from start() pending so the Retry
// button inside ObtainCertLive can try again; it rejects only when the
// dialog is closed without a successful issuance.
const issue = createRetryableIssue<CertificateResult>(() => {
  const live = refObtainCertLive.value
  if (!live)
    return Promise.reject(new Error('ObtainCertLive not mounted'))
  const { name, domains, key_type } = props.options
  return live.issue_cert(name!, domains, key_type)
})

watch(modalVisible, visible => {
  if (!visible)
    issue.cancel()
})

function start(): Promise<CertificateResult> {
  modalVisible.value = true
  return new Promise<CertificateResult>((resolve, reject) => {
    // Wait for the modal content to be rendered before issuing.
    nextTick(() => issue.start().then(resolve, reject))
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
      @retry="issue.retry"
    />
  </AModal>
</template>
