<script setup lang="ts">
import { useGettext } from 'vue3-gettext'
import type { SelectProps } from 'ant-design-vue'
import type { Ref } from 'vue'
import type { DNSProvider } from '@/api/auto_cert'
import auto_cert from '@/api/auto_cert'

const { $gettext } = useGettext()
const providers = ref([]) as Ref<DNSProvider[]>

// This data is provided by the Top StdCurd component,
// is the object that you are trying to modify it
const data = inject('data') as DNSProvider

const code = computed(() => {
  return data.code
})

const provider_idx = ref()
function init() {
  if (!data.configuration) {
    data.configuration = {
      credentials: {},
      additional: {},
    }
  }
  providers.value?.forEach((v: { code?: string }, k: number) => {
    if (v?.code === code.value)
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
  data.code = current.value.code
  data.provider = current.value.name

  auto_cert.get_dns_provider(current.value.code!).then(r => {
    Object.assign(current.value, r)
  })
})

const options = computed<SelectProps['options']>(() => {
  const list: SelectProps['options'] = []

  providers.value.forEach((v: DNSProvider, k: number) => {
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
