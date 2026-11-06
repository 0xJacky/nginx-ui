<script setup lang="ts">
import type { SelfSignedCertPayload } from '@/api/cert'
import SelfSignedCertFields from './SelfSignedCertFields.vue'

const data = defineModel<SelfSignedCertPayload>('value', { required: true })
</script>

<template>
  <ACard size="small" :title="$gettext('Self-signed Certificate')">
    <SelfSignedCertFields
      v-model="data"
      is-key-type-readonly
      hide-renewal-note
    />
  </ACard>
</template>
