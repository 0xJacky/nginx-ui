<script setup lang="ts">
import type { SelectProps } from 'ant-design-vue'
import type { DefaultOptionType } from 'ant-design-vue/es/select'
import type { Ref } from 'vue'
import type { AutoCertOptions, DNSProvider } from '@/api/auto_cert'
import auto_cert from '@/api/auto_cert'
import dns_credential from '@/api/dns_credential'

const providers = ref([]) as Ref<DNSProvider[]>
const credentials = ref<SelectProps['options']>([])

const data = defineModel<AutoCertOptions>('options', {
  default: () => {
    return {}
  },
  required: true,
})

const code = computed(() => {
  return data.value.code
})

const providerIdx = ref<number | undefined>(undefined)
function init() {
  providerIdx.value = undefined
  providers.value?.forEach((v: DNSProvider, k: number) => {
    if (v.code === code.value)
      providerIdx.value = k
  })
}

const current = computed(() => {
  const idx = providerIdx.value
  if (idx === undefined || idx === null)
    return undefined
  if (idx < 0 || idx >= providers.value.length)
    return undefined
  return providers.value?.[idx]
})

const mounted = ref(false)

watch(code, init)

watch(current, () => {
  if (!current.value) {
    credentials.value = []
    if (mounted.value) {
      data.value.code = undefined
      data.value.provider = undefined
      data.value.dns_credential_id = undefined
    }
    return
  }
  credentials.value = []
  data.value.code = current.value.code
  // Keep provider consistent with credential records (prefer provider/code over display name).
  data.value.provider = current.value.provider || current.value.code || current.value.name
  if (mounted.value)
    data.value.dns_credential_id = undefined

  dns_credential.getList({ provider: data.value.provider }).then(r => {
    r.data.forEach(v => {
      credentials.value?.push({
        value: v.id,
        label: v.name,
      })
    })
  })
})

onMounted(async () => {
  await auto_cert.get_dns_providers().then(r => {
    providers.value = r
  }).then(() => {
    init()
  })

  if (data.value.dns_credential_id) {
    await dns_credential.getItem(data.value.dns_credential_id).then(r => {
      const idx = providers.value.findIndex(v => v.code === r.code)
      if (idx > -1) {
        data.value.code = r.code
        data.value.provider = r.provider
        providerIdx.value = idx
      }
      else {
        // provider not supported anymore; clear existing selection to keep form consistent
        data.value.code = undefined
        data.value.provider = undefined
        data.value.dns_credential_id = undefined
        providerIdx.value = undefined
      }
    })
  }

  // prevent the dns_credential_id from being overwritten
  mounted.value = true
})

const options = computed<SelectProps['options']>(() => {
  const list: SelectProps['options'] = []

  providers.value.forEach((v, k: number) => {
    list!.push({
      value: k,
      label: v.name,
    })
  })

  return list
})

function filterOption(input: string, option?: DefaultOptionType) {
  return option?.label.toLowerCase().includes(input.toLowerCase())
}
</script>

<template>
  <AForm layout="vertical">
    <AFormItem :label="$gettext('DNS Provider')">
      <ASelect
        v-model:value="providerIdx"
        show-search
        :options
        :filter-option="filterOption"
      />
    </AFormItem>
    <AFormItem
      v-if="(providerIdx !== undefined && providerIdx !== null && providerIdx > -1)"
      :label="$gettext('Credential')"
      :rules="[{ required: true }]"
    >
      <ASelect
        v-model:value="(data.dns_credential_id as any)"
        :options="credentials"
      />
    </AFormItem>
  </AForm>
</template>

<style lang="less" scoped>

</style>
