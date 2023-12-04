<script setup lang="ts">
import { useGettext } from 'vue3-gettext'
import type { Ref } from 'vue'
import type { DnsChallenge } from '@/api/auto_cert'
import DNSChallenge from '@/views/domain/cert/components/DNSChallenge.vue'

defineProps<{
  hideNote?: boolean
}>()

const { $gettext } = useGettext()

const no_server_name = inject('no_server_name')

// Provide by ObtainCert.vue
const data = inject('data') as Ref<DnsChallenge>
</script>

<template>
  <div>
    <template v-if="no_server_name">
      <AAlert
        :message="$gettext('Warning')"
        type="warning"
        show-icon
      >
        <template #description>
          <span v-if="no_server_name">
            {{ $gettext('server_name parameter is required') }}
          </span>
        </template>
      </AAlert>
      <br>
    </template>

    <AAlert
      v-if="!hideNote"
      type="info"
      show-icon
      :message="$gettext('Note')"
      class="mb-4"
    >
      <template #description>
        <p>
          {{ $gettext('The server_name'
            + ' in the current configuration must be the domain name you need to get the certificate, support'
            + 'multiple domains.') }}
        </p>
        <p>
          {{ $gettext('The certificate for the domain will be checked 5 minutes, '
            + 'and will be renewed if it has been more than 1 week since it was last issued.') }}
        </p>
        <p v-if="data.challenge_method === 'http01'">
          {{ $gettext('Make sure you have configured a reverse proxy for .well-known '
            + 'directory to HTTPChallengePort before obtaining the certificate.') }}
        </p>
        <p v-else-if="data.challenge_method === 'dns01'">
          {{ $gettext('Please first add credentials in Certification > DNS Credentials, '
            + 'and then select one of the credentials'
            + 'below to request the API of the DNS provider.') }}
        </p>
      </template>
    </AAlert>
    <AForm layout="vertical">
      <AFormItem :label="$gettext('Challenge Method')">
        <ASelect v-model:value="data.challenge_method">
          <ASelectOption value="http01">
            {{ $gettext('HTTP01') }}
          </ASelectOption>
          <ASelectOption value="dns01">
            {{ $gettext('DNS01') }}
          </ASelectOption>
        </ASelect>
      </AFormItem>
    </AForm>
    <DNSChallenge v-if="data.challenge_method === 'dns01'" />
  </div>
</template>

<style lang="less" scoped>

</style>
