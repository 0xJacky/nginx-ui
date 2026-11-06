<script setup lang="ts">
import type { DnsCredential } from '@/api/dns_credential'
import DNSChallenge from './DNSChallenge.vue'

const data = defineModel<DnsCredential>('data', { required: true })
</script>

<template>
  <div>
    <AAlert
      class="mb-4"
      type="info"
      show-icon
      :title="$gettext('Note')"
    >
      <template #description>
        <p class="m-0">
          {{ $gettext('Please fill in the API authentication credentials provided by your DNS provider.') }}
        </p>
      </template>
    </AAlert>

    <AForm layout="vertical">
      <AFormItem :label="$gettext('Name')" required>
        <AInput v-model:value="data.name" />
      </AFormItem>

      <AFormItem :label="$gettext('Configuration')">
        <DNSChallenge v-model:data="data" />
      </AFormItem>
    </AForm>
  </div>
</template>
