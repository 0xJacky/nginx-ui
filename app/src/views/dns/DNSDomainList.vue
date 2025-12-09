<script setup lang="tsx">
import type { CustomRenderArgs, StdTableColumn } from '@uozi-admin/curd'
import type { SelectProps } from 'ant-design-vue'
import type { DNSProvider } from '@/api/auto_cert'
import type { DNSDomain } from '@/api/dns'
import { HeartOutlined, MailOutlined, StarOutlined } from '@ant-design/icons-vue'
import { datetimeRender, StdCurd } from '@uozi-admin/curd'
import { FormItem, Select } from 'ant-design-vue'
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import auto_cert from '@/api/auto_cert'
import { dnsApi } from '@/api/dns'
import dns_credential from '@/api/dns_credential'
import { filterAllowedDnsProviders } from '@/constants/dns_providers'

const router = useRouter()

type SelectOptionList = NonNullable<SelectProps['options']>

interface DomainForm extends Omit<DNSDomain, 'dns_credential_id'> {
  dns_credential_id?: number
  selected_provider?: string
  provider_initialized?: boolean
}

const dnsProviders = ref<DNSProvider[]>([])
const isLoadingProviders = ref(false)
const credentialOptions = reactive<Record<string, SelectOptionList>>({})
const credentialLoadingMap = reactive<Record<string, boolean>>({})

const providerOptions = computed<SelectOptionList>(() => {
  const list: SelectOptionList = []
  dnsProviders.value.forEach(provider => {
    const code = provider.code ?? provider.provider ?? ''
    const label = provider.name ?? provider.provider ?? code
    if (code) {
      list.push({
        label,
        value: code,
      })
    }
  })
  return list
})

onMounted(async () => {
  isLoadingProviders.value = true
  try {
    const list = await auto_cert.get_dns_providers()
    dnsProviders.value = filterAllowedDnsProviders(list)
  }
  finally {
    isLoadingProviders.value = false
  }
})

function clearCredentialSelection(formData: DomainForm) {
  formData.dns_credential_id = undefined
}

async function ensureCredentialOptions(providerCode: string) {
  if (!providerCode || credentialOptions[providerCode])
    return
  credentialLoadingMap[providerCode] = true
  try {
    const response = await dns_credential.getList({ provider_code: providerCode })
    const list = response?.data ?? []
    credentialOptions[providerCode] = list.map(item => ({
      label: item.name,
      value: item.id,
    }))
  }
  finally {
    credentialLoadingMap[providerCode] = false
  }
}

function filterOption(input: string, option?: { label?: string | number }) {
  if (!option?.label)
    return false
  return option.label.toString().toLowerCase().includes(input.toLowerCase())
}

interface DomainEditContext {
  formData: DomainForm
  mode: 'add' | 'edit'
}

function resolveProviderName(record: DomainForm) {
  return record.dns_credential?.provider
    ?? ''
}

function resolveProviderCode(record: DomainForm) {
  return record.dns_credential?.provider_code
    ?? ''
}

function resolveCredentialName(record: DomainForm) {
  return record.dns_credential?.name ?? '-'
}

const columns: StdTableColumn[] = [{
  title: () => $gettext('Domain'),
  dataIndex: 'domain',
  sorter: true,
  search: true,
  edit: {
    type: 'input',
    formItem: {
      required: true,
    },
  },
}, {
  title: () => $gettext('Credential'),
  dataIndex: 'dns_credential_id',
  customRender: ({ record }: CustomRenderArgs & { record: DNSDomain }) => {
    return resolveCredentialName(record as DomainForm)
  },
  edit: {
    type: (context: DomainEditContext) => {
      const formData = context.formData
      if (!formData.provider_initialized) {
        formData.selected_provider = context.mode === 'edit'
          ? (resolveProviderCode(formData) || resolveProviderName(formData))
          : ''
        formData.provider_initialized = true
      }
      const providerRef = computed({
        get: () => {
          return formData.selected_provider ?? ''
        },
        set: (value: string) => {
          formData.selected_provider = value
        },
      })

      const mergedProviderOptions = computed<SelectOptionList>(() => {
        const base = providerOptions.value
        if (
          providerRef.value
          && !base.some(option => option.value === providerRef.value)
        ) {
          return [
            ...base,
            {
              label: providerRef.value,
              value: providerRef.value,
            },
          ]
        }
        return base
      })

      if (providerRef.value)
        ensureCredentialOptions(providerRef.value)

      const credentialList: SelectOptionList = providerRef.value ? credentialOptions[providerRef.value] ?? [] : []
      const credentialField = providerRef.value
        ? (
            <FormItem key="credential" label={$gettext('Credential')} required>
              <Select
                v-model:value={formData.dns_credential_id}
                options={credentialList}
                placeholder={$gettext('Select Credential')}
                loading={Boolean(credentialLoadingMap[providerRef.value] && !credentialList.length)}
                showSearch
                filterOption={filterOption}
              />
            </FormItem>
          )
        : null

      function handleProviderChange(value: string) {
        providerRef.value = value
        clearCredentialSelection(formData)
        if (value)
          ensureCredentialOptions(value)
      }

      return (
        <div class="flex flex-col gap-4">
          <FormItem label={$gettext('Provider')} required>
            <Select
              value={providerRef.value || undefined}
              options={mergedProviderOptions.value}
              placeholder={$gettext('Select Provider')}
              showSearch
              filterOption={filterOption}
              loading={isLoadingProviders.value}
              onChange={handleProviderChange}
            />
          </FormItem>
          {credentialField}
        </div>
      )
    },
    formItem: {
      hiddenLabelInEdit: true,
    },
    rules: [{ required: true }],
  },
}, {
  title: () => $gettext('Provider'),
  dataIndex: 'credential.provider',
  customRender: ({ record }: CustomRenderArgs & { record: DNSDomain }) => {
    return resolveProviderName(record as DomainForm) || '--'
  },
  pure: true,
}, {
  title: () => $gettext('Updated at'),
  dataIndex: 'updated_at',
  customRender: datetimeRender,
  sorter: true,
  pure: true,
}, {
  title: () => $gettext('Actions'),
  dataIndex: 'actions',
  fixed: 'right',
}]

function goToRecords(record: DNSDomain) {
  router.push({
    name: 'DNS Domain Records',
    params: {
      id: record.id,
    },
  })
}
</script>

<template>
  <StdCurd
    :title="$gettext('DNS Domains')"
    :api="dnsApi"
    :columns
    disable-export
    disable-view
  >
    <template #beforeActions="{ record }">
      <AButton size="small" type="link" @click="goToRecords(record as DNSDomain)">
        {{ $gettext('Manage Records') }}
      </AButton>
    </template>

    <template #afterForm>
      <div class="mt-4 p-4 bg-gray-50 dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700">
        <ATypographyTitle :level="5" class="mb-3">
          <HeartOutlined class="mr-2 text-red-500" />
          {{ $gettext('DNS Provider Support') }}
        </ATypographyTitle>

        <div class="space-y-2">
          <div class="flex items-center space-x-2">
            <StarOutlined class="text-yellow-500 flex-shrink-0" />
            <ATypographyText class="text-sm">
              {{ $gettext('Need more DNS providers? Support us through donations or contact us for commercial collaboration') }}
            </ATypographyText>
          </div>
          <div class="flex items-center space-x-2">
            <MailOutlined class="text-blue-500 flex-shrink-0" />
            <ATypographyText class="text-sm">
              {{ $gettext('Business contact:') }}
              <a href="mailto:business@uozi.com" class="text-blue-600 hover:text-blue-800 dark:text-blue-400 dark:hover:text-blue-300">business@uozi.com</a>
            </ATypographyText>
          </div>
        </div>
      </div>
    </template>
  </StdCurd>
</template>

<style scoped lang="less">

</style>
