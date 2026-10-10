<script setup lang="ts">
import type { Rule } from 'antdv-next'
import type { SettingsField, SettingsSchema } from '@/api/plugin'
import { DeleteOutlined, PlusOutlined } from '@antdv-next/icons'
import { changedSettingKeys, translatePluginText as t } from './settingsForm'

const props = defineProps<{
  schema: SettingsSchema
  /** The stored values, what Discard goes back to. */
  saved: Record<string, unknown>
  saving?: boolean
}>()

const emit = defineEmits<{
  save: []
  discard: []
}>()

const values = defineModel<Record<string, unknown>>('values', {
  required: true,
})

const changedKeys = computed(() => new Set(changedSettingKeys(props.schema.settings, values.value, props.saved)))
const changedCount = computed(() => changedKeys.value.size)

function selectOptions(field: SettingsField) {
  return (field.options ?? []).map(option => ({
    value: option.value,
    label: t(option.label),
  }))
}

function listOf(key: string): string[] {
  const value = values.value[key]
  return Array.isArray(value) ? value as string[] : []
}

function setListItem(key: string, index: number, text: string) {
  const next = [...listOf(key)]
  next[index] = text
  values.value[key] = next
}

// Adds an empty row and moves the focus into it.
async function addListItem(key: string, event: MouseEvent) {
  const field = (event.currentTarget as HTMLElement | null)?.closest('.list-field')
  values.value[key] = [...listOf(key), '']
  await nextTick()
  const inputs = field?.querySelectorAll<HTMLInputElement>('.list-row input')
  inputs?.[inputs.length - 1]?.focus()
}

function removeListItem(key: string, index: number) {
  values.value[key] = listOf(key).filter((_, i) => i !== index)
}

const rules = computed(() => {
  return props.schema.settings.reduce((acc, field) => {
    if (!field.required)
      return acc

    const message = $gettext('%{field} is required', { field: t(field.display_name) })
    acc[field.key] = field.type === 'list'
      ? [{
          validator: async (_rule: Rule, value: unknown) => {
            const filled = Array.isArray(value) && value.some(item => typeof item === 'string' && item.trim())
            if (!filled)
              throw new Error(message)
          },
        }]
      : [{ required: true, message }]
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
      :title="t(props.schema.header)"
    />

    <AFormItem
      v-for="field in props.schema.settings"
      :key="field.key"
      :name="field.key"
      :extra="t(field.help_text) || undefined"
      :required="field.required"
    >
      <template #label>
        <span class="field-label">
          {{ t(field.display_name) }}
          <ATag v-if="changedKeys.has(field.key)" color="warning" class="m-0">
            {{ $gettext('Unsaved') }}
          </ATag>
        </span>
      </template>

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
      <div v-else-if="field.type === 'list'" class="list-field">
        <div
          v-for="(item, index) in listOf(field.key)"
          :key="index"
          class="list-row"
        >
          <AInput
            :value="item"
            @update:value="text => setListItem(field.key, index, String(text ?? ''))"
          />
          <AButton
            type="text"
            :aria-label="$gettext('Remove')"
            @click="removeListItem(field.key, index)"
          >
            <template #icon>
              <DeleteOutlined />
            </template>
          </AButton>
        </div>
        <div>
          <AButton @click="addListItem(field.key, $event)">
            <template #icon>
              <PlusOutlined />
            </template>
            {{ $gettext('Add') }}
          </AButton>
        </div>
      </div>
      <AInput
        v-else
        v-model:value="(values[field.key] as string)"
      />
    </AFormItem>

    <p v-if="props.schema.footer" class="text-gray-500">
      {{ t(props.schema.footer) }}
    </p>

    <div v-if="changedCount > 0" class="save-bar">
      <span class="save-bar-text">
        {{ $ngettext('%{count} unsaved', '%{count} unsaved', changedCount, { count: String(changedCount) }) }}
      </span>
      <div class="flex gap-2">
        <AButton :disabled="props.saving" @click="emit('discard')">
          {{ $gettext('Discard') }}
        </AButton>
        <AButton type="primary" html-type="submit" :loading="props.saving">
          {{ $gettext('Save') }}
        </AButton>
      </div>
    </div>
  </AForm>
</template>

<style lang="less" scoped>
.field-label {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}

.list-field {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.list-row {
  display: flex;
  align-items: center;
  gap: 4px;
}

// Stays in view at the bottom of the drawer while the form scrolls.
.save-bar {
  position: sticky;
  bottom: 0;
  z-index: 1;
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 8px 16px;
  padding: 12px 0;
  background: var(--ant-color-bg-elevated);
  border-top: 1px solid var(--ant-color-split);
}

.save-bar-text {
  color: var(--ant-color-text-secondary);
}
</style>
