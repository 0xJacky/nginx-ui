<script setup lang="ts">
import type { SelectProps } from 'ant-design-vue'
import type { DefaultOptionType } from 'ant-design-vue/es/select'
import type { Ref } from 'vue'
import type { DNSProvider } from '@/api/auto_cert'
import type { DnsCredential } from '@/api/dns_credential'
import auto_cert from '@/api/auto_cert'
import { isAllowedDnsProviderCode } from '@/constants/dns_providers'

const providers = ref([]) as Ref<DNSProvider[]>

// This data is provided by the Top StdCurd component,
// is the object that you are trying to modify it
const data = defineModel<DnsCredential>('data', { default: reactive({}) })

async function init() {
  if (!data.value.configuration) {
    data.value.configuration = {
      credentials: {},
      additional: {},
    }
  }
}

auto_cert.get_dns_providers().then(r => {
  providers.value = r
}).then(() => {
  init()
})

const current = computed(() => {
  return providers.value?.find(v => v.code === data.value.code)
})

const selectedProviderName = computed(() => {
  return current.value?.name ?? data.value.provider ?? data.value.code ?? ''
})

const isDnsRecordManagementSupported = computed(() => {
  return isAllowedDnsProviderCode(data.value.code)
})

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
  <AForm layout="vertical">
    <AFormItem :label="$gettext('DNS Provider')">
      <ASelect
        v-model:value="data.code"
        show-search
        :options="options"
        :filter-option="filterOption"
      />
      <AAlert
        class="mt-2"
        show-icon
        :type="dnsProviderHintType"
        :message="dnsProviderHint"
      />
    </AFormItem>
    <AFormItem>
      <!-- eslint-disable sonarjs/no-vue-bypass-sanitization -->
      <p v-if="current?.links?.api">
        {{ $gettext('API Document') }}: <a
          :href="current.links.api"
          target="_blank"
          rel="noopener noreferrer"
        >{{ current.links.api }}</a>
      </p>
      <p v-if="current?.links?.go_client">
        {{ $gettext('SDK') }}: <a
          :href="current.links.go_client"
          target="_blank"
          rel="noopener noreferrer"
        >{{ current.links.go_client }}</a>
      </p>
      <!-- eslint-enable -->
    </AFormItem>
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
  </AForm>
</template>

<style lang="less" scoped>

</style>
