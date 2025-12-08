<script setup lang="ts">
import type { RecordPayload } from '@/api/dns'
import { computed } from 'vue'

const props = defineProps<{
  showProxied?: boolean
  valueSuggestions?: string[]
}>()

const formModel = defineModel<RecordPayload>('record', {
  required: true,
  default: () => ({
    type: 'A',
    name: '@',
    content: '',
    ttl: 600,
  }),
})

const recordTypes = [
  'A',
  'AAAA',
  'CNAME',
  'TXT',
  'MX',
  'NS',
  'SRV',
  'CAA',
]

const showPriority = computed(() => {
  const type = formModel.value.type.toUpperCase()
  return ['MX', 'SRV'].includes(type)
})

const showWeight = computed(() => {
  const type = formModel.value.type.toUpperCase()
  return ['SRV'].includes(type)
})

const isValueAutocompleteEnabled = computed(() => {
  const type = formModel.value.type?.toUpperCase?.() ?? ''
  return type === 'A' || type === 'CNAME'
})

function handleValueKeydown(event: KeyboardEvent) {
  if (!isValueAutocompleteEnabled.value)
    return
  if (event.key === 'Enter') {
    event.preventDefault()
  }
}
</script>

<template>
  <AForm layout="vertical">
    <AFormItem :label="$gettext('Type')" :rules="[{ required: true }]">
      <ASelect
        v-model:value="formModel.type"
        :options="recordTypes.map(value => ({ label: value, value }))"
      />
    </AFormItem>
    <AFormItem :label="$gettext('Name')" :rules="[{ required: true }]">
      <AInput v-model:value="formModel.name" :placeholder="$gettext('Use @ for root')" />
    </AFormItem>
    <AFormItem :label="$gettext('Value')" :rules="[{ required: true }]">
      <AAutoComplete
        v-if="isValueAutocompleteEnabled"
        v-model:value="formModel.content"
        :options="(props.valueSuggestions ?? []).filter(Boolean).map(value => ({ value }))"
        :filter-option="(input, option) => option?.value?.toLowerCase().includes(input.toLowerCase()) ?? false"
        style="width: 100%;"
      >
        <ATextarea v-model:value="formModel.content" auto-size @keydown.enter="handleValueKeydown" />
      </AAutoComplete>
      <ATextarea v-else v-model:value="formModel.content" auto-size />
    </AFormItem>
    <AFormItem :label="$gettext('TTL (seconds)')" :rules="[{ required: true, type: 'number', min: 1 }]">
      <AInputNumber v-model:value="formModel.ttl" :min="1" :step="60" style="width: 100%;" />
    </AFormItem>
    <AFormItem v-if="showPriority" :label="$gettext('Priority')" :rules="[{ required: true, type: 'number', min: 0 }]">
      <AInputNumber v-model:value="formModel.priority" :min="0" style="width: 100%;" />
    </AFormItem>
    <AFormItem v-if="showWeight" :label="$gettext('Weight')">
      <AInputNumber v-model:value="formModel.weight" :min="0" :max="100" style="width: 100%;" />
    </AFormItem>
    <AFormItem v-if="props.showProxied" :label="$gettext('Proxied')">
      <ASwitch v-model:checked="formModel.proxied" />
    </AFormItem>
  </AForm>
</template>

<style scoped lang="less">

</style>
