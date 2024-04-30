<script setup lang="ts">
import type { SelectProps } from 'ant-design-vue'
import type { Ref } from 'vue'
import type { AcmeUser } from '@/api/acme_user'
import acme_user from '@/api/acme_user'
import type { Cert } from '@/api/cert'

const users = ref([]) as Ref<AcmeUser[]>

// This data is provided by the Top StdCurd component,
// is the object that you are trying to modify it
// we externalize the dns_credential_id to the parent component,
// this is used to tell the backend which dns_credential to use
const data = inject('data') as Ref<Cert>

const id = computed(() => {
  return data.value.acme_user_id
})

const user_idx = ref()
function init() {
  users.value?.forEach((v: AcmeUser, k: number) => {
    if (v.id === id.value)
      user_idx.value = k
  })
}

const current = computed(() => {
  return users.value?.[user_idx.value]
})

const mounted = ref(false)

watch(id, init)

watch(current, () => {
  data.value.acme_user_id = current.value.id
  if (!mounted.value)
    data.value.acme_user_id = 0
})

onMounted(async () => {
  await acme_user.get_list().then(r => {
    users.value = r.data
  }).then(() => {
    init()
  })

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

const filterOption = (input: string, option: { label: string }) => {
  return option.label.toLowerCase().includes(input.toLowerCase())
}
</script>

<template>
  <AForm layout="vertical">
    <AFormItem :label="$gettext('ACME User')">
      <ASelect
        v-model:value="user_idx"
        show-search
        :options="options"
        :filter-option="filterOption"
      />
    </AFormItem>
  </AForm>
</template>

<style lang="less" scoped>

</style>
