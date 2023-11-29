<script setup lang="ts">
import { useGettext } from 'vue3-gettext'
import type { SelectProps } from 'ant-design-vue'
import type { Ref } from 'vue'
import type { DNSProvider } from '@/api/auto_cert'
import auto_cert from '@/api/auto_cert'
import dns_credential from '@/api/dns_credential'

const { $gettext } = useGettext()
const providers = ref([]) as Ref<DNSProvider[]>
const credentials = ref<SelectProps['options']>([])

// This data is provided by the Top StdCurd component,
// is the object that you are trying to modify it
// we externalize the dns_credential_id to the parent component,
// this is used to tell the backend which dns_credential to use
const data = inject('data') as DNSProvider & { dns_credential_id: number | null }

const code = computed(() => {
  return data.code
})

const provider_idx = ref()
function init() {
  providers.value?.forEach((v: DNSProvider, k: number) => {
    if (v.code === code.value)
      provider_idx.value = k
  })
}

auto_cert.get_dns_providers().then(r => {
  providers.value = r
}).then(() => {
  init()
})

const current = computed(() => {
  return providers.value?.[provider_idx.value]
})

watch(code, init)

watch(current, () => {
  credentials.value = []
  data.code = current.value.code
  data.provider = current.value.name
  data.dns_credential_id = null

  dns_credential.get_list({ provider: data.provider }).then(r => {
    r.data.forEach(v => {
      credentials.value?.push({
        value: v.id,
        label: v.name,
      })
    })
  })
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

const filterOption = (input: string, option: { label: string }) => {
  return option.label.toLowerCase().includes(input.toLowerCase())
}
</script>

<template>
  <AForm layout="vertical">
    <AFormItem :label="$gettext('DNS Provider')">
      <ASelect
        v-model:value="provider_idx"
        show-search
        :options="options"
        :filter-option="filterOption"
      />
    </AFormItem>
    <AFormItem
      v-if="provider_idx > -1"
      :label="$gettext('Credential')"
      :rules="[{ required: true }]"
    >
      <ASelect
        v-model:value="data.dns_credential_id"
        :options="credentials"
      />
    </AFormItem>
  </AForm>
</template>

<style lang="less" scoped>

</style>
