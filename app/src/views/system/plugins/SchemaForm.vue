<script setup lang="ts">
import type { Rule } from 'antdv-next'
import type { SettingsField, SettingsSchema } from '@/api/plugin'

const props = defineProps<{
  schema: SettingsSchema
  saving?: boolean
}>()

const emit = defineEmits<{
  save: []
}>()

const values = defineModel<Record<string, unknown>>('values', {
  required: true,
})

function selectOptions(field: SettingsField) {
  return (field.options ?? []).map(option => ({
    value: option.value,
    label: option.label,
  }))
}

const rules = computed(() => {
  return props.schema.settings.reduce((acc, field) => {
    if (field.required)
      acc[field.key] = [{ required: true, message: $gettext('%{field} is required', { field: field.display_name }) }]

    return acc
  }, {} as Record<string, Rule[]>)
})
</script>

<template>
  <AForm
    layout="vertical"
    :model="values"
    :rules="rules"
    @finish="emit('save')"
  >
    <AAlert
      v-if="props.schema.header"
      type="info"
      show-icon
      class="mb-4"
      :title="props.schema.header"
    />

    <AFormItem
      v-for="field in props.schema.settings"
      :key="field.key"
      :name="field.key"
      :label="field.display_name"
      :extra="field.help_text"
      :required="field.required"
    >
      <ASwitch
        v-if="field.type === 'bool'"
        v-model:checked="(values[field.key] as boolean)"
      />
      <AInputNumber
        v-else-if="field.type === 'number'"
        v-model:value="(values[field.key] as number)"
        class="w-full"
      />
      <ASelect
        v-else-if="field.type === 'select'"
        v-model:value="(values[field.key] as string)"
        :options="selectOptions(field)"
        allow-clear
      />
      <AInputPassword
        v-else-if="field.type === 'secret'"
        v-model:value="(values[field.key] as string)"
        autocomplete="new-password"
      />
      <ATextarea
        v-else-if="field.type === 'textarea'"
        v-model:value="(values[field.key] as string)"
        :rows="4"
      />
      <AInput
        v-else
        v-model:value="(values[field.key] as string)"
      />
    </AFormItem>

    <p v-if="props.schema.footer" class="text-gray-500">
      {{ props.schema.footer }}
    </p>

    <AFormItem>
      <AButton type="primary" html-type="submit" :loading="props.saving">
        {{ $gettext('Save') }}
      </AButton>
    </AFormItem>
  </AForm>
</template>
