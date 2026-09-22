<script setup lang="ts">
import type { ConfigurationField } from '@/api/plugin'

// Renders the form a plugin declares for a notify channel, a probe kind, a
// storage backend or a deploy target.
// It only renders form items, so it must sit inside an AForm of the caller.
// Every value is a string, as on the wire: numbers in decimal, booleans as
// "true" or "false".
const props = defineProps<{
  fields: ConfigurationField[]
}>()

const values = defineModel<Record<string, string>>({ default: () => ({}) })

function update(key: string, value: string | number | null | undefined) {
  const next = { ...values.value }
  if (value === undefined || value === null || value === '')
    delete next[key]
  else
    next[key] = String(value)
  values.value = next
}

function numberValue(key: string): number | undefined {
  const raw = values.value[key]
  if (raw === undefined || raw === '')
    return undefined
  const parsed = Number(raw)
  return Number.isNaN(parsed) ? undefined : parsed
}

function onNumber(key: string, value: number | string | null) {
  update(key, value)
}

function onBool(key: string, checked: boolean | string | number) {
  update(key, checked === true ? 'true' : 'false')
}
</script>

<template>
  <AFormItem
    v-for="field in props.fields"
    :key="field.key"
    :label="field.display_name"
    :extra="field.help_text"
    :required="field.required"
  >
    <ASwitch
      v-if="field.type === 'bool'"
      :checked="values[field.key] === 'true'"
      @change="(checked: boolean | string | number) => onBool(field.key, checked)"
    />
    <AInputNumber
      v-else-if="field.type === 'number'"
      :value="numberValue(field.key)"
      class="w-full"
      @change="(value: number | string | null) => onNumber(field.key, value)"
    />
    <ATextarea
      v-else-if="field.type === 'textarea'"
      :value="values[field.key]"
      :rows="4"
      @update:value="(value?: string | number) => update(field.key, value)"
    />
    <AInputPassword
      v-else-if="field.secret"
      :value="values[field.key]"
      autocomplete="new-password"
      @update:value="(value?: string | number) => update(field.key, value)"
    />
    <AInput
      v-else
      :value="values[field.key]"
      @update:value="(value?: string | number) => update(field.key, value)"
    />
  </AFormItem>
</template>
