<script setup lang="ts">
import type { DNSRecordLine, RecordPayload } from '@/api/dns'
import { computed } from 'vue'

const props = defineProps<{
  showProxied?: boolean
  showComment?: boolean
  showLine?: boolean
  lineOptions?: DNSRecordLine[]
  isLineLoading?: boolean
  defaultLineCode?: string
  lineDisabled?: boolean
  valueSuggestions?: string[]
  showName?: boolean
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

const resolutionLineOptions = computed(() => {
  const lines = [...(props.lineOptions ?? [])]
  const currentLine = formModel.value.line?.trim()
  const defaultLineCode = props.defaultLineCode?.trim()

  if (defaultLineCode && !lines.some(line => line.code === defaultLineCode)) {
    lines.unshift({ code: defaultLineCode, display_name: $gettext('Default') })
  }
  if (currentLine && !lines.some(line => line.code === currentLine)) {
    lines.push({ code: currentLine, display_name: currentLine })
  }

  return lines.map(line => {
    const name = line.display_name || line.name || line.code
    return {
      label: name === line.code ? name : `${name} (${line.code})`,
      value: line.code,
    }
  })
})

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
    <AFormItem v-if="props.showName !== false" :label="$gettext('Name')" :rules="[{ required: true }]">
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
    <AFormItem v-if="props.showLine" :label="$gettext('Resolution Line')" :rules="[{ required: true }]">
      <ASelect
        v-model:value="formModel.line"
        :options="resolutionLineOptions"
        :loading="props.isLineLoading"
        :disabled="props.lineDisabled"
        show-search
        option-filter-prop="label"
      />
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
    <AFormItem v-if="props.showComment" :label="$gettext('Comment')">
      <ATextarea
        v-model:value="formModel.comment"
        :placeholder="$gettext('Optional comment for this DNS record')"
        :auto-size="{ minRows: 2, maxRows: 4 }"
      />
    </AFormItem>
  </AForm>
</template>

<style scoped lang="less">

</style>
