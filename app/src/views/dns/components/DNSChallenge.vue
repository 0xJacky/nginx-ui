<script setup lang="ts">
import type { SelectProps } from 'antdv-next'
import type { Ref } from 'vue'
import type { MethodStash } from './credentialForm'
import type { DNSProvider, DNSProviderField } from '@/api/auto_cert'
import type { DnsCredential } from '@/api/dns_credential'
import { ExportOutlined, RightOutlined } from '@antdv-next/icons'
import auto_cert from '@/api/auto_cert'
import Dns01PluginNotice from '@/components/Dns01PluginNotice'
import PluginSlot from '@/components/PluginSlot'
import { useDns01Plugin } from '@/composables/useDns01Plugin'
import { isAllowedDnsProviderCode } from '@/constants/dns_providers'
import {
  applyMethod,
  changedSettingsCount,
  configurationTarget,
  formMethods,
  initialMethod,
  methodNeedsNoInput,
  settingFields,
  visibleCredentialFields,
} from './credentialForm'

interface DefaultOptionType {
  label?: string
  value?: string
}

const providers = ref([]) as Ref<DNSProvider[]>
const providersLoaded = ref(false)

// This data is provided by the Top StdCurd component,
// is the object that you are trying to modify it
const data = defineModel<DnsCredential>('data', {
  default: () => ({}) as DnsCredential,
})

function ensureConfiguration() {
  data.value.configuration ??= { credentials: {}, additional: {} }
  data.value.configuration.credentials ??= {}
  data.value.configuration.additional ??= {}
}

function loadProviders() {
  return auto_cert.get_dns_providers().then(r => {
    providers.value = r
    providersLoaded.value = true
  })
}

loadProviders().then(() => {
  ensureConfiguration()
})

// Enabling or installing the plugin from the hint brings more providers.
const { state: dns01State } = useDns01Plugin()
watch(dns01State, (value, previous) => {
  if (value === 'available' && previous !== 'unknown')
    void loadProviders()
})

const current = computed(() => {
  return providers.value?.find(v => v.code === data.value.code)
})

const selectedProviderName = computed(() => {
  const name = current.value?.name ?? data.value.provider ?? data.value.code ?? ''
  return name ? $gettext(name) : ''
})

const isDnsRecordManagementSupported = computed(() => {
  // A provider contributed by a plugin reports the capability itself; fall back
  // to the built-in list for providers that predate the flag.
  if (typeof current.value?.record_management === 'boolean')
    return current.value.record_management

  return isAllowedDnsProviderCode(data.value.code)
})

const slotContext = computed(() => ({
  credential: data.value,
  provider: current.value,
}))

const dnsProviderHintType = computed(() => {
  if (!data.value.code)
    return 'info'

  return isDnsRecordManagementSupported.value ? 'success' : 'warning'
})

const dnsProviderHint = computed(() => {
  if (!data.value.code) {
    return $gettext('Select a DNS provider to see whether it supports DNS record management in DNS Domains.')
  }

  if (isDnsRecordManagementSupported.value) {
    return $gettext(
      '%{provider} can be used for ACME DNS-01 certificate challenges and DNS record management in DNS Domains.',
      { provider: selectedProviderName.value },
    )
  }

  return $gettext(
    '%{provider} can be used for ACME DNS-01 certificate challenges, but DNS record management in DNS Domains is not supported for this provider.',
    { provider: selectedProviderName.value },
  )
})

watch(current, () => {
  if (current.value) {
    data.value.code = current.value.code!
    data.value.provider = current.value.name!
    data.value.provider_code = current.value.code

    auto_cert.get_dns_provider(data.value.code).then(r => {
      Object.assign(current.value!, r)
    })
  }
}, { immediate: true })

const options = computed<SelectProps['options']>(() => {
  return providers.value.map(v => ({
    value: v.code,
    label: v.name ? $gettext(v.name) : v.code,
  }))
})

function filterOption(input: string, option?: DefaultOptionType) {
  const needle = input.toLowerCase()
  const label = option?.label?.toString().toLowerCase() ?? ''
  const value = option?.value?.toString().toLowerCase() ?? ''
  return label.includes(needle) || value.includes(needle)
}

// Structured form

const form = computed(() => current.value?.form)
const hasForm = computed(() => !!form.value?.fields?.length || formMethods(form.value).length > 0)
const methods = computed(() => formMethods(form.value))
const selectedMethod = ref<string>()
let stash: MethodStash = {}

const methodOptions = computed(() => methods.value.map(method => ({
  value: method.name,
  label: method.recommended
    ? $gettext('%{method} (recommended)', { method: $gettext(method.name) })
    : $gettext(method.name),
})))

// Pick the method once the provider schema arrives, or when the provider or
// the edited credential changes.
watch([() => data.value, () => data.value.code, form], ([record, code], [previousRecord, previousCode]) => {
  if (record !== previousRecord || code !== previousCode)
    stash = {}
  if (!form.value) {
    selectedMethod.value = undefined
    return
  }
  ensureConfiguration()
  selectedMethod.value = initialMethod(form.value, data.value.configuration.credentials)
  applyMethod(data.value.configuration.credentials, form.value, selectedMethod.value, stash)
}, { immediate: true })

function selectMethod(value: string | number) {
  selectedMethod.value = String(value)
  ensureConfiguration()
  applyMethod(data.value.configuration.credentials, form.value, selectedMethod.value, stash)
}

const credentialFieldsShown = computed(() => visibleCredentialFields(form.value, selectedMethod.value))
const needsNoInput = computed(() => methodNeedsNoInput(form.value, selectedMethod.value))
const settings = computed(() => settingFields(form.value))
const settingsOpen = ref(false)

function fieldValue(field: DNSProviderField) {
  const target = configurationTarget(field)
  return data.value.configuration?.[target]?.[field.key]
}

function setFieldValue(field: DNSProviderField, value: string) {
  ensureConfiguration()
  const map = data.value.configuration[configurationTarget(field)]
  if (value === '')
    delete map[field.key]
  else
    map[field.key] = value
}

const changedCount = computed(() => changedSettingsCount(settings.value, fieldValue))

function fieldLabel(field: DNSProviderField) {
  return $gettext(field.label || field.key)
}
</script>

<template>
  <div class="dns-challenge-form">
    <!-- Nothing follows the provider until one is chosen, so drop the gap. -->
    <AFormItem :label="$gettext('DNS Provider')" :class="{ 'mb-0': !current }">
      <ASelect
        v-model:value="data.code"
        show-search
        :options="options"
        :filter-option="filterOption"
      />
      <!-- eslint-disable sonarjs/no-vue-bypass-sanitization -->
      <a
        v-if="current?.links?.api"
        class="inline-flex items-center gap-1 mt-1 text-sm"
        :href="current.links.api"
        target="_blank"
        rel="noopener noreferrer"
      >
        {{ $gettext('%{provider} API docs', { provider: selectedProviderName }) }}
        <ExportOutlined />
      </a>
      <!-- eslint-enable -->
      <Dns01PluginNotice
        v-if="providersLoaded"
        class="mt-2"
        variant="providers"
        :provider-count="providers.length"
      />
      <AAlert
        class="mt-2"
        show-icon
        :type="dnsProviderHintType"
        :title="dnsProviderHint"
      />
    </AFormItem>
    <PluginSlot
      :name="`dns.credential.hint:${data.code}`"
      :context="slotContext"
    />
    <PluginSlot
      :name="`dns.credential.form:${data.code}`"
      :context="slotContext"
    >
      <template v-if="hasForm && data.configuration">
        <AFormItem
          v-if="methods.length"
          :label="$gettext('Sign-in method')"
          :extra="needsNoInput ? $gettext('Nothing to fill in. The server uses its own credentials.') : undefined"
        >
          <ASegmented
            :value="selectedMethod"
            :options="methodOptions"
            class="max-w-full overflow-x-auto"
            @change="selectMethod"
          />
        </AFormItem>

        <AFormItem
          v-for="field in credentialFieldsShown"
          :key="field.key"
          :required="!field.optional"
          :extra="field.help ? $gettext(field.help) : undefined"
        >
          <template #label>
            <span class="field-label">
              <span>{{ fieldLabel(field) }}</span>
              <ATag v-if="field.optional" class="m-0" :bordered="false">
                {{ $gettext('Optional') }}
              </ATag>
              <span class="field-key">{{ field.key }}</span>
              <!-- eslint-disable sonarjs/no-vue-bypass-sanitization -->
              <a
                v-if="field.link"
                :href="field.link"
                target="_blank"
                rel="noopener noreferrer"
                :title="$gettext('Details')"
                :aria-label="$gettext('Details')"
              >
                <ExportOutlined />
              </a>
              <!-- eslint-enable -->
            </span>
          </template>
          <AInputPassword
            v-if="field.secret"
            :value="fieldValue(field)"
            :placeholder="field.default || undefined"
            autocomplete="new-password"
            @update:value="v => setFieldValue(field, v ?? '')"
          />
          <AInput
            v-else
            :value="fieldValue(field)"
            :placeholder="field.default || undefined"
            @update:value="v => setFieldValue(field, v ?? '')"
          />
        </AFormItem>

        <template v-if="settings.length">
          <button
            type="button"
            class="settings-toggle"
            :aria-expanded="settingsOpen"
            @click="settingsOpen = !settingsOpen"
          >
            <span class="flex items-center gap-2">
              <RightOutlined class="settings-arrow" :class="{ 'is-open': settingsOpen }" />
              {{ $gettext('Advanced Settings') }}
            </span>
            <span v-if="changedCount" class="settings-summary is-changed">
              {{ $gettext('%{count} changed', { count: changedCount }) }}
            </span>
            <span v-else class="settings-summary">
              {{ $gettext('All default') }}
            </span>
          </button>
          <div v-show="settingsOpen" class="settings-panel">
            <AFormItem
              v-for="field in settings"
              :key="field.key"
              :extra="field.help ? $gettext(field.help) : undefined"
            >
              <template #label>
                <span class="field-label">
                  <span>{{ fieldLabel(field) }}</span>
                  <span class="field-key">{{ field.key }}</span>
                  <!-- eslint-disable sonarjs/no-vue-bypass-sanitization -->
                  <a
                    v-if="field.link"
                    :href="field.link"
                    target="_blank"
                    rel="noopener noreferrer"
                    :title="$gettext('Details')"
                    :aria-label="$gettext('Details')"
                  >
                    <ExportOutlined />
                  </a>
                  <!-- eslint-enable -->
                </span>
              </template>
              <AInputPassword
                v-if="field.secret"
                :value="fieldValue(field)"
                :placeholder="field.default || undefined"
                autocomplete="new-password"
                @update:value="v => setFieldValue(field, v ?? '')"
              />
              <AInput
                v-else
                :value="fieldValue(field)"
                :placeholder="field.default || undefined"
                :suffix="field.unit === 'seconds' ? $gettext('sec') : undefined"
                :inputmode="field.unit === 'seconds' ? 'numeric' : undefined"
                @update:value="v => setFieldValue(field, v ?? '')"
              />
            </AFormItem>
          </div>
        </template>
      </template>
    </PluginSlot>
  </div>
</template>

<style lang="less" scoped>
.field-label {
  display: inline-flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 4px 8px;
}

.field-key {
  font-family: var(--ant-font-family-code, ui-monospace, SFMono-Regular, Menlo, monospace);
  font-size: 12px;
  color: var(--ant-color-text-tertiary);
  word-break: break-all;
}

.settings-toggle {
  display: flex;
  width: 100%;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 16px;
  padding: 12px 0 0;
  border: 0;
  border-top: 1px solid var(--ant-color-split);
  background: transparent;
  color: var(--ant-color-text);
  font: inherit;
  cursor: pointer;
  text-align: left;
}

.settings-arrow {
  font-size: 12px;
  transition: transform 0.2s;

  &.is-open {
    transform: rotate(90deg);
  }
}

.settings-summary {
  font-size: 12px;
  color: var(--ant-color-text-tertiary);

  &.is-changed {
    color: var(--ant-color-warning);
  }
}

.settings-panel {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  column-gap: 16px;
  margin-bottom: 16px;
  padding: 16px 16px 0;
  border-radius: var(--ant-border-radius-lg);
  background: var(--ant-color-fill-quaternary);
}

@media (min-width: 576px) {
  .settings-panel {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
</style>
