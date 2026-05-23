<script setup lang="ts">
import type { CertificateInfo, SelfSignedCertPayload } from '@/api/cert'
import CertInfo from '@/components/CertInfo'
import SelfSignedCertFields from './SelfSignedCertFields.vue'

defineProps<{
  certificateInfo?: CertificateInfo
}>()

const data = defineModel<SelfSignedCertPayload>('value', { required: true })
</script>

<template>
  <div class="self-signed-cert-management mb-4">
    <AAlert
      class="mb-4"
      :message="$gettext('This self-signed certificate is managed by Nginx UI and renewed automatically.')"
      type="success"
      show-icon
    />
    <AForm
      v-if="certificateInfo"
      layout="vertical"
    >
      <AFormItem :label="$gettext('Certificate Status')">
        <CertInfo
          :cert="certificateInfo"
          class="max-w-96"
        />
      </AFormItem>
    </AForm>
    <SelfSignedCertFields
      v-model="data"
      is-key-type-readonly
    />
  </div>
</template>
