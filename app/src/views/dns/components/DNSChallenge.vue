<script setup lang="ts">
import type { SelectProps } from 'antdv-next'
import type { Ref } from 'vue'
import type { DNSProvider } from '@/api/auto_cert'
import type { DnsCredential } from '@/api/dns_credential'
import auto_cert from '@/api/auto_cert'
import Dns01PluginNotice from '@/components/Dns01PluginNotice'
import PluginSlot from '@/components/PluginSlot'
import { useDns01Plugin } from '@/composables/useDns01Plugin'
import { isAllowedDnsProviderCode } from '@/constants/dns_providers'

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

async function init() {
  if (!data.value.configuration) {
    data.value.configuration = {
      credentials: {},
      additional: {},
    }
  }
}

function loadProviders() {
  return auto_cert.get_dns_providers().then(r => {
    providers.value = r
    providersLoaded.value = true
  })
}

loadProviders().then(() => {
  init()
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
  return current.value?.name ?? data.value.provider ?? data.value.code ?? ''
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
    label: v.name,
  }))
})

function filterOption(input: string, option?: DefaultOptionType) {
  const needle = input.toLowerCase()
  const label = option?.label?.toString().toLowerCase() ?? ''
  const value = option?.value?.toString().toLowerCase() ?? ''
  return label.includes(needle) || value.includes(needle)
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
    <AFormItem v-if="current?.links?.api || current?.links?.go_client">
      <!-- eslint-disable sonarjs/no-vue-bypass-sanitization -->
      <p v-if="current?.links?.api" class="m-0">
        {{ $gettext('API Document') }}: <a
          :href="current.links.api"
          target="_blank"
          rel="noopener noreferrer"
        >{{ current.links.api }}</a>
      </p>
      <p v-if="current?.links?.go_client" class="m-0 mt-1">
        {{ $gettext('SDK') }}: <a
          :href="current.links.go_client"
          target="_blank"
          rel="noopener noreferrer"
        >{{ current.links.go_client }}</a>
      </p>
      <!-- eslint-enable -->
    </AFormItem>
    <PluginSlot
      :name="`dns.credential.hint:${data.code}`"
      :context="slotContext"
    />
    <PluginSlot
      :name="`dns.credential.form:${data.code}`"
      :context="slotContext"
    >
      <template v-if="current?.configuration?.credentials">
        <h4>{{ $gettext('Credentials') }}</h4>
        <AFormItem
          v-for="(v, k) in current?.configuration?.credentials"
          :key="k"
          :label="k"
          :extra="v"
        >
          <AInput v-model:value="data.configuration.credentials[k]" />
        </AFormItem>
      </template>
      <template v-if="current?.configuration?.additional">
        <h4>{{ $gettext('Additional') }}</h4>
        <AFormItem
          v-for="(v, k) in current?.configuration?.additional"
          :key="k"
          :label="k"
          :extra="v"
        >
          <AInput v-model:value="data.configuration.additional[k]" />
        </AFormItem>
      </template>
    </PluginSlot>
  </div>
</template>

<style lang="less" scoped>

</style>
