<script setup lang="ts">
import type { SelectProps } from 'antdv-next'
import type { RecordListParams } from '@/api/dns'

const emit = defineEmits<{
  (event: 'submit'): void
}>()

const filters = defineModel<RecordListParams>('filters', {
  default: () => ({
    type: '',
    name: '',
  }),
})

const recordTypes = [
  '',
  'A',
  'AAAA',
  'CNAME',
  'TXT',
  'MX',
  'NS',
  'SRV',
  'CAA',
]

const recordTypeOptions = computed<SelectProps['options']>(() => recordTypes.map(type => ({
  label: type || $gettext('All'),
  value: type,
})))

function handleSubmit() {
  emit('submit')
}

function handleReset() {
  filters.value = {
    type: '',
    name: '',
  }
  emit('submit')
}
</script>

<template>
  <AForm layout="inline" @submit.prevent>
    <AFormItem :label="$gettext('Host')">
      <AInput
        v-model:value="filters.name"
        :placeholder="$gettext('Host, e.g. @ or www')"
        style="width: 200px;"
      />
    </AFormItem>
    <AFormItem :label="$gettext('Type')">
      <ASelect
        v-model:value="filters.type"
        :options="recordTypeOptions"
        style="width: 160px;"
      />
    </AFormItem>
    <AFormItem>
      <ASpace>
        <AButton type="primary" @click="handleSubmit">
          {{ $gettext('Search') }}
        </AButton>
        <AButton @click="handleReset">
          {{ $gettext('Reset') }}
        </AButton>
      </ASpace>
    </AFormItem>
  </AForm>
</template>

<style scoped lang="less">

</style>
