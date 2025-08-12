<script setup lang="ts">
import type { AcmeUser } from '@/api/acme_user'
import type { AutoCertOptions } from '@/api/auto_cert'
import acme_user from '@/api/acme_user'

const data = defineModel<AutoCertOptions>('options', {
  default: reactive({}),
  required: true,
})

const users = ref<AcmeUser[]>([])
const loading = ref(false)

// Load ACME users on component mount
onMounted(async () => {
  loading.value = true
  try {
    users.value = []
    let page = 1
    while (true) {
      try {
        const r = await acme_user.getList({ page })
        users.value.push(...r.data)
        if (r?.data?.length < (r?.pagination?.per_page ?? 0))
          break
        page++
      }
      catch {
        break
      }
    }
  }
  finally {
    loading.value = false
  }
})

// Define field names mapping for ASelect
const fieldNames = {
  value: 'id',
  label: 'name',
}

// Filter function for search - using type assertion for compatibility
function filterOption(input: string, option?: unknown) {
  return (option as AcmeUser)?.name?.toLowerCase().includes(input.toLowerCase()) ?? false
}

const value = computed({
  set(value: number) {
    data.value.acme_user_id = value
  },
  get() {
    if (data.value.acme_user_id && data.value.acme_user_id > 0) {
      return data.value.acme_user_id
    }
    return undefined
  },
})
</script>

<template>
  <AForm layout="vertical">
    <AFormItem :label="$gettext('ACME User')">
      <ASelect
        v-model:value="value"
        :placeholder="$gettext('System Initial User')"
        :loading="loading"
        show-search
        :options="users"
        :field-names="fieldNames"
        :filter-option="filterOption"
      />
    </AFormItem>
  </AForm>
</template>

<style lang="less" scoped>

</style>
