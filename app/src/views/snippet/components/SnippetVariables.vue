<script setup lang="ts">
import type { VariableOption, VariableRow, VariableType } from '../variables'
import { DeleteOutlined, DownOutlined, PlusOutlined, RightOutlined } from '@antdv-next/icons'
import { localize, useSnippetDescription } from '../description'
import { colorOf, colorSlots, undeclaredColor } from '../template'
import { keyProblem, newOption, newVariable, optionsProblem } from '../variables'
import LocalizedInput from './LocalizedInput.vue'

const props = defineProps<{
  // Variable keys the content refers to.
  referenced: Set<string>
}>()

const rows = defineModel<VariableRow[]>({ required: true })
// Key of the open variable, shared with the content editor.
const activeKey = defineModel<string>('active')

const { current: language } = useSnippetDescription()

const openId = ref<number>()
const slots = computed(() => colorSlots(rows.value))

const typeOptions = computed(() => [
  { label: $gettext('Text'), value: 'string' },
  { label: $gettext('Switch'), value: 'boolean' },
  { label: $gettext('Select'), value: 'select' },
])

const typeLabels = computed(() => Object.fromEntries(typeOptions.value.map(o => [o.value, o.label])) as Record<VariableType, string>)

const undeclared = computed(() => {
  const keys = new Set(rows.value.map(r => r.key.trim()))
  return [...props.referenced].filter(key => !keys.has(key))
})

// The list and the content editor open the same variable.
watch(activeKey, key => {
  const row = rows.value.find(r => r.key.trim() === key)
  if (row)
    openId.value = row.id
})

const openRow = computed(() => rows.value.find(r => r.id === openId.value))

watch(() => openRow.value?.key, key => {
  if (key !== undefined)
    activeKey.value = key.trim()
})

function toggle(row: VariableRow) {
  if (openId.value === row.id) {
    openId.value = undefined
    activeKey.value = undefined
    return
  }
  openId.value = row.id
  activeKey.value = row.key.trim()
}

function addVariable(key = '') {
  const row = newVariable(key)
  rows.value.push(row)
  openId.value = row.id
  activeKey.value = key
}

function removeVariable(row: VariableRow) {
  rows.value = rows.value.filter(r => r !== row)
  if (openId.value === row.id) {
    openId.value = undefined
    activeKey.value = undefined
  }
}

function changeType(row: VariableRow, type: VariableType) {
  row.type = type
  if (type === 'boolean') {
    row.value = false
  }
  else if (type === 'select') {
    if (row.options.length === 0)
      row.options.push(newOption())
    row.value = row.options[0].value
  }
  else {
    row.value = ''
  }
}

// The default follows its option when the value of the option changes.
function changeOptionValue(row: VariableRow, option: VariableOption, value: string) {
  if (row.value === option.value)
    row.value = value
  option.value = value
}

function removeOption(row: VariableRow, option: VariableOption) {
  row.options = row.options.filter(o => o !== option)
  if (row.value === option.value)
    row.value = row.options[0]?.value ?? ''
}

function optionLabel(option: VariableOption) {
  return localize(option.labels, language.value) || option.value
}

function defaultOptions(row: VariableRow) {
  return row.options
    .filter(o => o.value.trim())
    .map(o => ({ label: optionLabel(o), value: o.value }))
}

function defaultOf(row: VariableRow) {
  if (row.type === 'boolean')
    return row.value ? $gettext('On') : $gettext('Off')
  if (row.type === 'select') {
    const option = row.options.find(o => o.value === row.value)
    return option ? optionLabel(option) : ''
  }
  return String(row.value)
}

function problemOf(row: VariableRow) {
  return keyProblem(row, rows.value) || optionsProblem(row)
}

function isUnused(row: VariableRow) {
  return !keyProblem(row, rows.value) && !props.referenced.has(row.key.trim())
}

function colorFor(row: VariableRow) {
  return keyProblem(row, rows.value) ? 'var(--ant-color-border)' : colorOf(slots.value, row.key.trim())
}
</script>

<template>
  <div class="flex flex-col gap-2">
    <div
      v-for="row in rows"
      :key="row.id"
      class="variable"
      :class="{ open: openId === row.id, invalid: !!problemOf(row) }"
      :style="{ '--c': colorFor(row) }"
    >
      <button
        type="button"
        class="summary"
        :aria-expanded="openId === row.id"
        @click="toggle(row)"
      >
        <span class="flex min-w-0 flex-1 flex-col">
          <span class="truncate font-medium">{{ localize(row.names, language) || row.key || $gettext('New Variable') }}</span>
          <span
            v-if="row.key.trim()"
            class="key truncate font-mono text-xs"
          >.{{ row.key.trim() }}</span>
        </span>
        <span class="flex shrink-0 items-center gap-1">
          <ATag
            v-if="isUnused(row)"
            color="warning"
            :bordered="false"
            class="m-0"
          >
            {{ $gettext('Not Used') }}
          </ATag>
          <ATag
            :bordered="false"
            class="m-0"
          >
            {{ typeLabels[row.type] }}
          </ATag>
          <span class="hint max-w-24 truncate text-xs">{{ defaultOf(row) }}</span>
          <component
            :is="openId === row.id ? DownOutlined : RightOutlined"
            class="hint text-[10px]"
          />
        </span>
      </button>

      <div
        v-if="openId === row.id"
        class="body flex flex-col gap-3"
      >
        <div class="grid grid-cols-2 gap-3">
          <label class="flex min-w-0 flex-col gap-1">
            <span class="hint text-[13px]">{{ $gettext('Key') }}</span>
            <AInput
              v-model:value="row.key"
              class="font-mono"
              placeholder="maxAge"
              :status="row.key && keyProblem(row, rows) ? 'error' : undefined"
            />
          </label>
          <label class="flex min-w-0 flex-col gap-1">
            <span class="hint text-[13px]">{{ $gettext('Label') }}</span>
            <LocalizedInput
              v-model="row.names"
              :maxlength="100"
              :placeholder="$gettext('Max Age')"
            />
          </label>
          <label class="flex min-w-0 flex-col gap-1">
            <span class="hint text-[13px]">{{ $gettext('Type') }}</span>
            <ASelect
              :value="row.type"
              :options="typeOptions"
              @change="changeType(row, $event as VariableType)"
            />
          </label>
          <label class="flex min-w-0 flex-col gap-1">
            <span class="hint text-[13px]">{{ $gettext('Default Value') }}</span>
            <div
              v-if="row.type === 'boolean'"
              class="flex h-8 items-center"
            >
              <ASwitch v-model:checked="row.value as boolean" />
            </div>
            <ASelect
              v-else-if="row.type === 'select'"
              v-model:value="row.value as string"
              :options="defaultOptions(row)"
            />
            <AInput
              v-else
              v-model:value="row.value as string"
            />
          </label>
        </div>

        <div
          v-if="row.type === 'select'"
          class="flex flex-col gap-2"
        >
          <span class="hint text-[13px]">{{ $gettext('Options') }}</span>
          <div
            v-for="option in row.options"
            :key="option.id"
            class="grid grid-cols-[minmax(0,1fr)_minmax(0,1.5fr)_auto] gap-2"
          >
            <AInput
              :value="option.value"
              class="font-mono"
              :placeholder="$gettext('Value')"
              :aria-label="$gettext('Value')"
              @update:value="changeOptionValue(row, option, $event)"
            />
            <LocalizedInput
              v-model="option.labels"
              :maxlength="100"
              :placeholder="$gettext('Label')"
            />
            <AButton
              type="text"
              :aria-label="$gettext('Remove option')"
              :disabled="row.options.length === 1"
              @click="removeOption(row, option)"
            >
              <template #icon>
                <DeleteOutlined />
              </template>
            </AButton>
          </div>
          <div>
            <AButton
              size="small"
              @click="row.options.push(newOption())"
            >
              <template #icon>
                <PlusOutlined />
              </template>
              {{ $gettext('Add Option') }}
            </AButton>
          </div>
        </div>

        <div
          v-if="problemOf(row)"
          class="problem text-sm"
        >
          {{ problemOf(row) }}
        </div>

        <div class="flex justify-end">
          <AButton
            size="small"
            danger
            @click="removeVariable(row)"
          >
            <template #icon>
              <DeleteOutlined />
            </template>
            {{ $gettext('Remove variable') }}
          </AButton>
        </div>
      </div>
    </div>

    <div
      v-for="key in undeclared"
      :key="`undeclared-${key}`"
      class="variable undeclared"
      :style="{ '--c': undeclaredColor }"
    >
      <div class="summary">
        <span class="flex min-w-0 flex-1 flex-col">
          <span class="key truncate font-mono">.{{ key }}</span>
          <span class="hint text-xs">{{ $gettext('Used in the content, not declared') }}</span>
        </span>
        <AButton
          size="small"
          type="link"
          @click="addVariable(key)"
        >
          {{ $gettext('Declare Variable') }}
        </AButton>
      </div>
    </div>

    <div>
      <AButton
        size="small"
        @click="addVariable()"
      >
        <template #icon>
          <PlusOutlined />
        </template>
        {{ $gettext('Add Variable') }}
      </AButton>
    </div>
  </div>
</template>

<style scoped lang="less">
.variable {
  border: 1px solid var(--ant-color-border-secondary);
  border-left: 3px solid var(--c);
  border-radius: 8px;
  background: var(--ant-color-bg-container);

  &.open {
    border-color: var(--ant-color-primary-border);
    border-left-color: var(--c);
    box-shadow: 0 0 0 2px var(--ant-color-primary-bg);
  }

  &.invalid:not(.open) {
    border-color: var(--ant-color-error-border);
    border-left-color: var(--c);
  }

  &.undeclared {
    border-style: dashed;
    border-left-style: solid;
    background: var(--ant-color-warning-bg);
  }
}

.summary {
  display: flex;
  width: 100%;
  align-items: center;
  gap: 8px;
  padding: 6px 10px;
  border: 0;
  background: transparent;
  color: inherit;
  font: inherit;
  text-align: left;
  cursor: pointer;
}

.undeclared .summary {
  cursor: default;
}

.body {
  padding: 4px 10px 10px;
}

.key {
  color: color-mix(in srgb, var(--c) 75%, var(--ant-color-text));
}

.hint {
  color: var(--ant-color-text-secondary);
}

.problem {
  color: var(--ant-color-error);
}
</style>
