<script setup lang="ts">
import type { AutoCertOptions } from '@/api/auto_cert'
import { AutoCertChallengeMethod } from '@/api/auto_cert'
import { PrivateKeyTypeEnum, PrivateKeyTypeList } from '@/constants'
import { isIPAddress } from '@/utils/certificate'
import ACMEUserSelector from '@/views/certificate/components/ACMEUserSelector.vue'
import DNSChallenge from './DNSChallenge.vue'

const props = defineProps<{
  hideNote?: boolean
  forceDnsChallenge?: boolean
  keyTypeReadOnly?: boolean
  isDefaultServer?: boolean
  hasWildcardServerName?: boolean
  isIpCertificate?: boolean
  needsManualIpInput?: boolean
}>()

const data = defineModel<AutoCertOptions>('options', {
  default: reactive({}),
  required: true,
})

const manualIpAddress = defineModel<string>('manualIpAddress', { default: '' })

onMounted(() => {
  if (!data.value.key_type)
    data.value.key_type = PrivateKeyTypeEnum.P256

  if (props.forceDnsChallenge)
    data.value.challenge_method = AutoCertChallengeMethod.dns01
  else if (props.isIpCertificate || props.needsManualIpInput)
    data.value.challenge_method = AutoCertChallengeMethod.http01
})

watch(() => props.forceDnsChallenge, v => {
  if (v)
    data.value.challenge_method = AutoCertChallengeMethod.dns01
})

watch(() => [props.isIpCertificate, props.needsManualIpInput], ([isIpCertificate, needsManualIpInput]) => {
  if ((isIpCertificate || needsManualIpInput) && !props.forceDnsChallenge)
    data.value.challenge_method = AutoCertChallengeMethod.http01
})

// IP address validation function
function validateIpAddress(_rule: unknown, value: string) {
  if (!value || value.trim() === '') {
    return Promise.reject($gettext('Please enter the server IP address'))
  }

  const trimmedValue = value.trim()
  if (!isIPAddress(trimmedValue)) {
    return Promise.reject($gettext('Please enter a valid IPv4 or IPv6 address'))
  }

  return Promise.resolve()
}

async function validateManualIpAddress() {
  if (props.needsManualIpInput)
    await validateIpAddress(undefined, manualIpAddress.value)
}

defineExpose({
  validateManualIpAddress,
})
</script>

<template>
  <div>
    <!-- IP Certificate Warning -->
    <AAlert
      v-if="(isIpCertificate || needsManualIpInput) && !hideNote"
      type="warning"
      show-icon
      :message="$gettext('IP Certificate Notice')"
      class="mb-4"
    >
      <template #description>
        <p v-if="isDefaultServer">
          {{ $gettext('This site is configured as a default server (default_server) for HTTPS (port 443). '
            + 'IP certificates require Certificate Authority (CA) support and may not be available with all ACME providers.') }}
        </p>
        <p v-else-if="hasWildcardServerName">
          {{ $gettext('This site uses wildcard server name (_) which typically indicates an IP-based certificate. '
            + 'IP certificates require Certificate Authority (CA) support and may not be available with all ACME providers.') }}
        </p>
        <p v-if="needsManualIpInput">
          {{ $gettext('No specific IP address found in server_name configuration. '
            + 'Please specify the server IP address below for the certificate.') }}
        </p>
        <p>
          {{ $gettext('For IP-based certificate configurations, only HTTP-01 challenge method is supported. '
            + 'DNS-01 challenge is not compatible with IP-based certificates.') }}
        </p>
      </template>
    </AAlert>

    <AAlert
      v-if="!hideNote && !isIpCertificate && !needsManualIpInput"
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
          {{ $gettext('The certificate for the domain is checked every 30 minutes and renewed when its remaining validity reaches the threshold configured in settings.') }}
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
      <!-- IP Address Input for IP certificates without explicit IP -->
      <AFormItem
        v-if="needsManualIpInput"
        :label="$gettext('Server IP Address')"
        :rules="[{ validator: validateIpAddress, trigger: 'blur' }]"
      >
        <AInput
          v-model:value="manualIpAddress"
          :placeholder="$gettext('Enter server IP address (e.g., 203.0.113.1 or 2001:db8::1)')"
        />
        <template #help>
          <div class="space-y-2">
            <p>
              {{ $gettext('For IP-based certificates, please specify the server IP address that will be included in the certificate.') }}
            </p>
            <div class="text-xs text-gray-600">
              <p class="font-medium">
                {{ $gettext('Public CA Requirements:') }}
              </p>
              <ul class="ml-4 list-disc space-y-1">
                <li>
                  {{ $gettext('Must be a public IP address accessible from the internet') }}
                </li>
                <li>
                  {{ $gettext('Port 80 must be open for HTTP-01 challenge validation') }}
                </li>
                <li>
                  {{ $gettext('Private IPs (192.168.x.x, 10.x.x.x, 172.16-31.x.x) will fail') }}
                </li>
              </ul>
              <p class="mt-2 font-medium">
                {{ $gettext('Private CA:') }}
              </p>
              <p class="ml-4">
                {{ $gettext('Any reachable IP address can be used with private Certificate Authorities') }}
              </p>
            </div>
          </div>
        </template>
      </AFormItem>

      <AFormItem
        v-if="!forceDnsChallenge"
        :label="$gettext('Challenge Method')"
      >
        <ASelect v-model:value="data.challenge_method">
          <ASelectOption value="http01">
            {{ $gettext('HTTP01') }}
          </ASelectOption>
          <ASelectOption
            value="dns01"
            :disabled="isIpCertificate || needsManualIpInput"
          >
            {{ $gettext('DNS01') }}
            <span v-if="isIpCertificate || needsManualIpInput" class="text-gray-400 ml-2">
              ({{ $gettext('Not supported for IP certificates') }})
            </span>
          </ASelectOption>
        </ASelect>
      </AFormItem>
      <AFormItem
        :label="$gettext('Key Type')"
      >
        <ASelect
          v-model:value="data.key_type"
          :disabled="keyTypeReadOnly"
        >
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
      <AFormItem :label="$gettext('Enable Common Name')">
        <template #help>
          <p>
            {{ $gettext('Enable the certificate Common Name field for private CAs that still require it.') }}
          </p>
        </template>
        <ASwitch v-model:checked="data.enable_common_name" />
      </AFormItem>
      <AFormItem :label="$gettext('Revoke Old Certificate')">
        <template #help>
          <p>
            {{ $gettext('If you want to automatically revoke the old certificate, please enable this option.') }}
          </p>
        </template>
        <ASwitch v-model:checked="data.revoke_old" />
      </AFormItem>
    </AForm>
  </div>
</template>

<style lang="less" scoped>

</style>
