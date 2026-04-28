<script setup lang="ts">
import type { SelectProps } from 'ant-design-vue'
import type { DefaultOptionType } from 'ant-design-vue/es/select'
import type { AutoCertOptions } from '@/api/auto_cert'
import type { DnsCredential } from '@/api/dns_credential'
import { InfoCircleOutlined } from '@ant-design/icons-vue'
import { useRouter } from 'vue-router'
import dns_credential from '@/api/dns_credential'

const router = useRouter()

const data = defineModel<AutoCertOptions>('options', {
  default: () => {
    return {}
  },
  required: true,
})

const loading = ref(false)
const credentials = ref<DnsCredential[]>([])
const credentialOptions = ref<SelectProps['options']>([])

function resolveProviderLabel(item: DnsCredential) {
  return item.provider || item.provider_code || item.code || $gettext('Unknown Provider')
}

function mapCredentialOption(item: DnsCredential): NonNullable<SelectProps['options']>[number] {
  return {
    value: item.id,
    label: `${item.name} (${resolveProviderLabel(item)})`,
  }
}

function applyCredentialMeta(item?: DnsCredential) {
  if (!item) {
    data.value.dns_credential_id = undefined
    data.value.code = undefined
    data.value.provider = undefined
    data.value.provider_code = undefined
    return
  }

  data.value.dns_credential_id = item.id
  data.value.code = item.code
  data.value.provider = item.provider
  data.value.provider_code = item.provider_code || item.code
}

function onCredentialChange(value?: number) {
  const current = credentials.value.find(item => item.id === value)
  applyCredentialMeta(current)
}

const selectedCredentialId = computed<SelectProps['value']>({
  get: () => {
    return data.value.dns_credential_id ?? undefined
  },
  set: value => {
    let selectedID: number | undefined
    if (typeof value === 'number')
      selectedID = value
    else if (typeof value === 'string')
      selectedID = Number(value)

    if (selectedID !== undefined && Number.isNaN(selectedID))
      selectedID = undefined

    onCredentialChange(selectedID)
  },
})

async function loadCredentials() {
  loading.value = true
  try {
    credentials.value = []
    let page = 1

    while (true) {
      try {
        const r = await dns_credential.getList({ page })
        const list = r?.data ?? []
        credentials.value.push(...list)

        const perPage = r?.pagination?.per_page ?? 0
        if (!perPage || list.length < perPage)
          break

        page++
      }
      catch {
        break
      }
    }

    credentialOptions.value = credentials.value.map(mapCredentialOption)

    if (data.value.dns_credential_id) {
      const current = credentials.value.find(item => item.id === data.value.dns_credential_id)
      if (current)
        applyCredentialMeta(current)
      else
        applyCredentialMeta(undefined)
    }
  }
  finally {
    loading.value = false
  }
}

function goToCredentialPage() {
  router.push('/dns/credentials')
}

function filterOption(input: string, option?: DefaultOptionType) {
  const needle = input.toLowerCase()
  const label = option?.label?.toString().toLowerCase() ?? ''
  const value = option?.value?.toString().toLowerCase() ?? ''
  return label.includes(needle) || value.includes(needle)
}

onMounted(async () => {
  await loadCredentials()
})
</script>

<template>
  <AForm layout="vertical">
    <AFormItem :rules="[{ required: true }]">
      <template #label>
        <span>{{ $gettext('Credential') }}</span>
        <ATooltip :title="$gettext('Please create DNS credentials first in DNS > Credentials')">
          <InfoCircleOutlined class="ml-2 cursor-pointer text-gray-500" @click="goToCredentialPage" />
        </ATooltip>
      </template>
      <ASelect
        v-model:value="selectedCredentialId"
        :options="credentialOptions"
        :placeholder="$gettext('Select Credential')"
        :loading="loading"
        show-search
        :filter-option="filterOption"
      />
      <AButton type="link" size="small" class="px-0" @click="goToCredentialPage">
        {{ $gettext('Go to DNS > Credentials to create or manage credentials') }}
      </AButton>
    </AFormItem>
  </AForm>
</template>

<style lang="less" scoped>

</style>
