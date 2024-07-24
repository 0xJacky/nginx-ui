<script setup lang="ts">
import { $gettext } from '../../../../gettext'
import type { AutoCertOptions } from '@/api/auto_cert'
import DNSChallenge from '@/views/domain/cert/components/DNSChallenge.vue'
import ACMEUserSelector from '@/views/certificate/ACMEUserSelector.vue'
import { PrivateKeyTypeList } from '@/constants'

const props = defineProps<{
  hideNote?: boolean
  forceDnsChallenge?: boolean
}>()

const data = defineModel<AutoCertOptions>('options', {
  default: () => {
    return {}
  },
  required: true,
})

onMounted(() => {
  if (!data.value.key_type)
    data.value.key_type = '2048'

  if (props.forceDnsChallenge)
    data.value.challenge_method = 'dns01'
})

watch(() => props.forceDnsChallenge, v => {
  if (v)
    data.value.challenge_method = 'dns01'
})
</script>

<template>
  <div>
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
          {{ $gettext('The certificate for the domain will be checked 30 minutes, '
            + 'and will be renewed if it has been more than 1 week or the period you set in settings since it was last issued.') }}
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
      <AFormItem
        v-if="!forceDnsChallenge"
        :label="$gettext('Challenge Method')"
      >
        <ASelect v-model:value="data.challenge_method">
          <ASelectOption value="http01">
            {{ $gettext('HTTP01') }}
          </ASelectOption>
          <ASelectOption value="dns01">
            {{ $gettext('DNS01') }}
          </ASelectOption>
        </ASelect>
      </AFormItem>
      <AFormItem :label="$gettext('Key Type')">
        <ASelect v-model:value="data.key_type">
          <ASelectOption
            v-for="t in PrivateKeyTypeList"
            :key="t.key"
            :value="t.key"
          >
            {{ t.name }}
          </ASelectOption>
        </ASelect>
      </AFormItem>
    </AForm>
    <ACMEUserSelector v-model:options="data" />
    <DNSChallenge
      v-if="data.challenge_method === 'dns01'"
      v-model:options="data"
    />
    <AForm layout="vertical">
      <AFormItem :label="$gettext('OCSP Must Staple')">
        <template #help>
          <p>
            {{ $gettext('Do not enable this option unless you are sure that you need it.') }}
            {{ $gettext('OCSP Must Staple may cause errors for some users on first access using Firefox.') }}
            <a href="https://github.com/0xJacky/nginx-ui/issues/322">#322</a>
          </p>
        </template>
        <ASwitch v-model:checked="data.must_staple" />
      </AFormItem>
      <AFormItem :label="$gettext('Lego disable CNAME Support')">
        <template #help>
          <p>
            {{ $gettext('If your domain has CNAME records and you cannot obtain certificates, '
              + 'you need to enable this option.') }}
          </p>
        </template>
        <ASwitch v-model:checked="data.lego_disable_cname_support" />
      </AFormItem>
    </AForm>
  </div>
</template>

<style lang="less" scoped>

</style>
