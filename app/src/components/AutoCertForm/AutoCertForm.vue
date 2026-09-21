<script setup lang="ts">
import type { SelectProps } from 'antdv-next'
import type { AutoCertOptions, ChallengeMethod } from '@/api/auto_cert'
import { useRouter } from 'vue-router'
import auto_cert, { AutoCertChallengeMethod } from '@/api/auto_cert'
import PluginSlot from '@/components/PluginSlot'
import { PrivateKeyTypeEnum, PrivateKeyTypeList } from '@/constants'
import { isIPAddress } from '@/utils/certificate'
import ACMEUserSelector from '@/views/certificate/components/ACMEUserSelector.vue'
import InstallConfirmModal from '@/views/system/plugins/marketplace/InstallConfirmModal.vue'
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

/** The official plugin that contributes the DNS-01 challenge. */
const DNS01_PLUGIN_ID = 'com.nginxui.dns01'

const data = defineModel<AutoCertOptions>('options', {
  required: true,
})

const manualIpAddress = defineModel<string>('manualIpAddress', { default: '' })

const router = useRouter()

const challengeMethods = ref<ChallengeMethod[]>([])
// Until the backend answers, assume every method is available so the form does
// not flash a warning for a plugin that is in fact installed.
const challengeMethodsLoaded = ref(false)

const availableMethods = computed(() => new Set(challengeMethods.value.map(method => method.code)))

function isMethodAvailable(code: keyof typeof AutoCertChallengeMethod) {
  return !challengeMethodsLoaded.value || availableMethods.value.has(code)
}

// DNS-01 is contributed by a plugin, so it can legitimately be missing.
const isDns01Available = computed(() => isMethodAvailable(AutoCertChallengeMethod.dns01))

const challengeMethodOptions = computed<SelectProps['options']>(() => [
  {
    value: AutoCertChallengeMethod.http01,
    disabled: !isMethodAvailable(AutoCertChallengeMethod.http01),
    label: $gettext('HTTP01'),
  },
  {
    value: AutoCertChallengeMethod.dns01,
    disabled: props.isIpCertificate || props.needsManualIpInput || !isDns01Available.value,
    label: $gettext('DNS01'),
  },
])

async function loadChallengeMethods() {
  try {
    challengeMethods.value = await auto_cert.get_challenge_methods()
  }
  catch {
    // Keep both options usable rather than blocking issuance on a failed probe.
    challengeMethods.value = []
    return
  }

  challengeMethodsLoaded.value = true
}

function goToPluginsPage() {
  router.push('/system/plugins')
}

// Installing the plugin from here saves a trip to System > Plugins, which is
// the only other place that offers it.
const dns01InstallOpen = ref(false)

function installDns01Plugin() {
  dns01InstallOpen.value = true
}

async function onDns01PluginInstalled() {
  challengeMethodsLoaded.value = false
  await loadChallengeMethods()
}

const keyTypeOptions: SelectProps['options'] = PrivateKeyTypeList.map(t => ({
  key: t.key,
  value: t.key,
  label: t.name,
}))

const compactLabelCol = { flex: '170px' }
const compactWrapperCol = { flex: 'auto' }

onMounted(() => {
  void loadChallengeMethods()

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
    <ACard size="small" class="cert-config-card mb-4" :title="$gettext('Required Settings')">
      <AForm
        layout="horizontal"
        label-align="left"
        :label-col="compactLabelCol"
        :wrapper-col="compactWrapperCol"
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

        <AAlert
          v-if="!isDns01Available"
          class="mb-4"
          type="warning"
          show-icon
          :title="$gettext('DNS-01 challenge requires the DNS-01 plugin. Install it from System > Plugins.')"
        >
          <template #description>
            <ASpace wrap>
              <AButton type="primary" size="small" @click="installDns01Plugin">
                {{ $gettext('Install DNS-01 plugin') }}
              </AButton>
              <AButton type="link" size="small" class="px-0" @click="goToPluginsPage">
                {{ $gettext('Go to System > Plugins') }}
              </AButton>
            </ASpace>
          </template>
        </AAlert>

        <InstallConfirmModal
          v-model:open="dns01InstallOpen"
          :plugin-id="DNS01_PLUGIN_ID"
          @installed="onDns01PluginInstalled"
        />
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
                v-if="option.data.value === AutoCertChallengeMethod.dns01 && (isIpCertificate || needsManualIpInput)"
                class="text-gray-400 ml-2"
              >
                ({{ $gettext('Not supported for IP certificates') }})
              </span>
            </template>
            <template #labelRender="{ label, value }">
              {{ label }}
              <span
                v-if="value === AutoCertChallengeMethod.dns01 && (isIpCertificate || needsManualIpInput)"
                class="text-gray-400 ml-2"
              >
                ({{ $gettext('Not supported for IP certificates') }})
              </span>
            </template>
          </ASelect>
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

      <ACMEUserSelector v-model:options="data" compact />
      <PluginSlot
        v-if="data.challenge_method === 'dns01'"
        :name="`certificate.challenge.form:${data.challenge_method}`"
        :context="{ options: data }"
      >
        <div class="mt-4">
          <DNSChallenge v-model:options="data" compact />
        </div>
      </PluginSlot>
    </ACard>

    <ACard size="small" class="cert-config-card" :title="$gettext('Special Settings')">
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
    </ACard>
    <PluginSlot name="certificate.issue.footer" :context="{ options: data }" />
  </div>
</template>

<style lang="less" scoped>
.cert-config-card {
  :deep(.ant-card-head) {
    min-height: 40px;
  }

  :deep(.ant-card-body) {
    padding: 14px;
  }
}
</style>
