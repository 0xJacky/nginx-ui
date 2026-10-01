<script setup lang="ts">
import type { SelectProps } from 'antdv-next'
import type { AutoCertOptions } from '@/api/auto_cert'
import type { DnsVerifyOn } from '@/composables/useMainNodeDns01'
import { useMediaQuery } from '@vueuse/core'
import { AutoCertChallengeMethod } from '@/api/auto_cert'
import Dns01PluginNotice from '@/components/Dns01PluginNotice'
import PluginSlot from '@/components/PluginSlot'
import { useDns01Plugin } from '@/composables/useDns01Plugin'
import { useMainNodeDns01 } from '@/composables/useMainNodeDns01'
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
  /** The certificate covers a wildcard name, which HTTP-01 cannot validate. */
  wildcard?: boolean
  /** Editing a certificate that already exists, as opposed to issuing one. */
  existing?: boolean
}>()

const data = defineModel<AutoCertOptions>('options', {
  required: true,
})

const manualIpAddress = defineModel<string>('manualIpAddress', { default: '' })

// A credential id of 0 means none. Plugins only ever see undefined for it.
if (data.value.dns_credential_id === 0 || data.value.dns_credential_id === null)
  data.value.dns_credential_id = undefined

const dns01 = useDns01Plugin()
const { state: dns01State } = dns01

const isIpOnly = computed(() => !!(props.isIpCertificate || props.needsManualIpInput))

const isWildcard = computed(() => !!props.wildcard
  || (data.value.domains ?? []).some(domain => domain?.startsWith('*.')))

const isDns01 = computed(() => data.value.challenge_method === AutoCertChallengeMethod.dns01)

const { canVerifyOnMain, mainDns01, verifyOptions, verifyHint: hintFor } = useMainNodeDns01(() => !props.existing)
const verifyOn = computed<DnsVerifyOn>({
  get: () => data.value.verify_on ?? 'main',
  set: value => {
    data.value.verify_on = value
  },
})
const isOnMainNode = computed(() => canVerifyOnMain.value && isDns01.value && verifyOn.value === 'main')
const verifyHint = computed(() => hintFor(verifyOn.value))

/** DNS-01 is selected but nothing can run it right now. */
const isDns01Blocked = computed(() => isDns01.value && (isOnMainNode.value
  ? mainDns01.value === 'missing'
  : !dns01.isAvailable.value))

const challengeMethodOptions = computed<SelectProps['options']>(() => [
  {
    value: AutoCertChallengeMethod.http01,
    label: $gettext('HTTP01'),
  },
  {
    value: AutoCertChallengeMethod.dns01,
    disabled: isIpOnly.value,
    label: $gettext('DNS01'),
  },
])

const challengeHint = computed<{ text: string, warning?: boolean } | undefined>(() => {
  if (isDns01.value)
    return isDns01Blocked.value ? undefined : { text: $gettext('Validates the domain with a DNS record. Needs a DNS credential.') }
  if (isWildcard.value)
    return { text: $gettext('Wildcard certificates usually need DNS-01. HTTP-01 is likely to fail.'), warning: true }
  return { text: $gettext('Validates each domain over port 80 on this server.') }
})

const readonlyCredentialHelp = computed(() => dns01State.value === 'missing'
  ? $gettext('Can be changed after the plugin is installed.')
  : $gettext('Can be changed after the plugin is enabled.'))

// Phones stack labels above the controls; the plugin block follows via compact.
const isNarrow = useMediaQuery('(max-width: 575px)')
const compact = computed(() => !isNarrow.value)
const slotContext = computed(() => ({ options: data.value, compact: compact.value }))

const keyTypeOptions: SelectProps['options'] = PrivateKeyTypeList.map(t => ({
  key: t.key,
  value: t.key,
  label: t.name,
}))

const compactLabelCol = { flex: '170px' }
const compactWrapperCol = { flex: '1 1 0', style: { minWidth: 0 } }

const cardStyles = {
  header: { minHeight: '40px' },
  body: { padding: '14px' },
}

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
  /** True while DNS-01 is selected but cannot run, so issuing would fail. */
  dns01Blocked: isDns01Blocked,
})
</script>

<template>
  <div>
    <!-- IP Certificate Warning -->
    <AAlert
      v-if="(isIpCertificate || needsManualIpInput) && !hideNote"
      type="warning"
      show-icon
      :title="$gettext('IP Certificate Notice')"
      class="mb-4"
    >
      <template #description>
        <p v-if="isDefaultServer">
          {{ $gettext('This site is configured as a default server (default_server) for HTTPS (port 443). IP certificates require Certificate Authority (CA) support and may not be available with all ACME providers.') }}
        </p>
        <p v-else-if="hasWildcardServerName">
          {{ $gettext('This site uses wildcard server name (_) which typically indicates an IP-based certificate. IP certificates require Certificate Authority (CA) support and may not be available with all ACME providers.') }}
        </p>
        <p v-if="needsManualIpInput">
          {{ $gettext('No specific IP address found in server_name configuration. Please specify the server IP address below for the certificate.') }}
        </p>
        <p>
          {{ $gettext('For IP-based certificate configurations, only HTTP-01 challenge method is supported. DNS-01 challenge is not compatible with IP-based certificates.') }}
        </p>
      </template>
    </AAlert>

    <AAlert
      v-if="!hideNote && !isIpCertificate && !needsManualIpInput"
      type="info"
      show-icon
      :title="$gettext('Note')"
      class="mb-4"
    >
      <template #description>
        <p>
          {{ $gettext('The server_name in the current configuration must be the domain name you need to get the certificate, support multiple domains.') }}
        </p>
        <p>
          {{ $gettext('The certificate for the domain is checked every 30 minutes and renewed when its remaining validity reaches the threshold configured in settings.') }}
        </p>
        <p v-if="data.challenge_method === 'http01'">
          {{ $gettext('Make sure you have configured a reverse proxy for .well-known directory to HTTPChallengePort before obtaining the certificate.') }}
        </p>
        <p v-else-if="data.challenge_method === 'dns01'">
          {{ $gettext('Please first add credentials in Certification > DNS Credentials, and then select one of the credentials below to request the API of the DNS provider.') }}
        </p>
      </template>
    </AAlert>
    <ACard size="small" class="mb-4" :styles="cardStyles" :title="$gettext('Issuing')">
      <Dns01PluginNotice
        v-if="forceDnsChallenge && isDns01Blocked && !isOnMainNode"
        class="mb-4"
        :variant="existing ? 'renewal' : 'challenge'"
      />
      <AForm
        :layout="compact ? 'horizontal' : 'vertical'"
        :label-align="compact ? 'left' : undefined"
        :label-col="compact ? compactLabelCol : undefined"
        :wrapper-col="compact ? compactWrapperCol : undefined"
        :model="{ manualIpAddress }"
      >
        <!-- IP Address Input for IP certificates without explicit IP -->
        <AFormItem
          v-if="needsManualIpInput"
          name="manualIpAddress"
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
          <ASelect
            v-model:value="data.challenge_method"
            :options="challengeMethodOptions"
          >
            <template #optionRender="{ option }">
              {{ option.data.label }}
              <span
                v-if="option.data.value === AutoCertChallengeMethod.dns01 && isIpOnly"
                class="challenge-hint ml-2"
              >
                ({{ $gettext('Not supported for IP certificates') }})
              </span>
            </template>
            <template #labelRender="{ label, value }">
              {{ label }}
              <span
                v-if="value === AutoCertChallengeMethod.dns01 && isIpOnly"
                class="challenge-hint ml-2"
              >
                ({{ $gettext('Not supported for IP certificates') }})
              </span>
            </template>
          </ASelect>
          <div
            v-if="challengeHint"
            class="challenge-hint mt-1"
            :class="{ 'is-warning': challengeHint.warning }"
          >
            {{ challengeHint.text }}
          </div>
          <Dns01PluginNotice
            v-if="isDns01Blocked && !isOnMainNode"
            class="mt-2"
            :variant="existing ? 'renewal' : 'challenge'"
          />
        </AFormItem>
        <AFormItem
          v-if="canVerifyOnMain && isDns01"
          :label="$gettext('DNS Validation')"
        >
          <ASegmented
            v-model:value="verifyOn"
            :options="verifyOptions"
          />
          <div class="challenge-hint mt-1">
            {{ verifyHint }}
          </div>
          <AAlert
            v-if="isOnMainNode && mainDns01 === 'missing'"
            class="mt-2"
            type="warning"
            show-icon
            :title="$gettext('The main node has no DNS-01 plugin. Install it on the main node, or validate on this node.')"
          />
        </AFormItem>
        <AFormItem
          :label="$gettext('Key Type')"
        >
          <ASelect
            v-model:value="data.key_type"
            :disabled="keyTypeReadOnly"
            :options="keyTypeOptions"
          />
        </AFormItem>
      </AForm>

      <ACMEUserSelector v-model:options="data" :compact="compact" />
      <template v-if="isDns01">
        <DNSChallenge
          v-if="isOnMainNode"
          v-model:options="data"
          :compact="compact"
          main-node
        />
        <PluginSlot
          v-else-if="!isDns01Blocked"
          :name="`certificate.challenge.form:${data.challenge_method}`"
          :context="slotContext"
        >
          <DNSChallenge v-model:options="data" :compact="compact" />
        </PluginSlot>
        <!-- Without the plugin an existing certificate keeps showing its credential. -->
        <DNSChallenge
          v-else-if="existing"
          v-model:options="data"
          :compact="compact"
          readonly
          :readonly-help="readonlyCredentialHelp"
        />
      </template>
    </ACard>

    <ACard size="small" :styles="cardStyles" :title="$gettext('Certificate options')">
      <div class="cert-option-row">
        <div class="cert-option-text">
          <div>{{ $gettext('OCSP Must Staple') }}</div>
          <div class="cert-option-desc">
            {{ $gettext('Only turn on when you are sure you need it. Firefox may show an error on the first visit.') }}
            <a href="https://github.com/0xJacky/nginx-ui/issues/322" target="_blank" rel="noopener noreferrer">#322</a>
          </div>
        </div>
        <ASwitch v-model:checked="data.must_staple" :aria-label="$gettext('OCSP Must Staple')" />
      </div>
      <div class="cert-option-row">
        <div class="cert-option-text">
          <div>{{ $gettext('Enable Common Name') }}</div>
          <div class="cert-option-desc">
            {{ $gettext('For private CAs that still require this field.') }}
          </div>
        </div>
        <ASwitch v-model:checked="data.enable_common_name" :aria-label="$gettext('Enable Common Name')" />
      </div>
      <div class="cert-option-row">
        <div class="cert-option-text">
          <div>{{ $gettext('Revoke Old Certificate') }}</div>
          <div class="cert-option-desc">
            {{ $gettext('Revokes the previous certificate once the new one is issued.') }}
          </div>
        </div>
        <ASwitch v-model:checked="data.revoke_old" :aria-label="$gettext('Revoke Old Certificate')" />
      </div>
    </ACard>
    <PluginSlot name="certificate.issue.footer" :context="{ options: data }" />
  </div>
</template>

<style lang="less" scoped>
.challenge-hint {
  font-size: 13px;
  color: var(--ant-color-text-secondary);

  &.is-warning {
    color: var(--ant-color-warning-text);
  }
}

.cert-option-row {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 10px 0;

  & + & {
    border-top: 1px solid var(--ant-color-split);
  }

  &:first-child {
    padding-top: 0;
  }

  &:last-child {
    padding-bottom: 0;
  }
}

.cert-option-text {
  flex: 1;
  min-width: 0;
}

.cert-option-desc {
  margin-top: 2px;
  font-size: 13px;
  color: var(--ant-color-text-secondary);
}
</style>
