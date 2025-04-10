<script setup lang="ts">
import type { AcmeUser } from '@/api/acme_user'
import type { AutoCertOptions } from '@/api/auto_cert'
import type { SelectProps } from 'ant-design-vue'
import type { Ref } from 'vue'
import acme_user from '@/api/acme_user'

const users = ref([]) as Ref<AcmeUser[]>

const data = defineModel<AutoCertOptions>('options', {
  default: () => {
    return {}
  },
  required: true,
})

const id = computed(() => {
  return data.value?.acme_user_id
})

const userIdx = ref<number>()
function init() {
  users.value?.forEach((v: AcmeUser, k: number) => {
    if (v.id === id.value)
      userIdx.value = k
  })
}

const current = computed(() => {
  return users.value?.[userIdx.value || -1]
})

const mounted = ref(false)

watch(id, init)

watch(current, () => {
  if (mounted.value)
    data.value!.acme_user_id = current.value.id
})

onMounted(async () => {
  users.value = []
  let page = 1
  while (true) {
    try {
      const r = await acme_user.get_list({ page })

      users.value.push(...r.data)
      if (r?.data?.length < (r?.pagination?.per_page ?? 0))
        break
      page++
    }
    catch {
      break
    }
  }

  init()

  // prevent the acme_user_id from being overwritten
  mounted.value = true
})

const options = computed<SelectProps['options']>(() => {
  const list: SelectProps['options'] = []

  users.value.forEach((v, k: number) => {
    list!.push({
      value: k,
      label: v.name,
    })
  })

  return list
})

function filterOption(input: string, option: { label: string }) {
  return option.label.toLowerCase().includes(input.toLowerCase())
}
</script>

<template>
  <AForm layout="vertical">
    <AFormItem :label="$gettext('ACME User')">
      <ASelect
        v-model:value="userIdx"
        :placeholder="$gettext('System Initial User')"
        show-search
        :options
        :filter-option="filterOption"
      />
    </AFormItem>
  </AForm>
</template>

<style lang="less" scoped>

</style>
