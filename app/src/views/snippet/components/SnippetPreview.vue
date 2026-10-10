<script setup lang="ts">
import type { VariableRow } from '../variables'
import type { Variable } from '@/api/template'
import { UndoOutlined } from '@antdv-next/icons'
import { createReusableTemplate, watchDebounced } from '@vueuse/core'
import snippet from '@/api/snippet'
import { localize, useSnippetDescription } from '../description'
import { referencedKeys } from '../template'
import { hasVariableProblems, toVariables } from '../variables'
import CodeView from './CodeView.vue'

const props = defineProps<{
  rows: VariableRow[]
  content: string
  // The include directive, for a snippet without variables.
  include?: string
  // The content is a template of a plugin.
  fromPlugin?: boolean
}>()

const { current: language } = useSnippetDescription()
const [DefineStatus, Status] = createReusableTemplate()

const isIncludable = computed(() => props.rows.length === 0 && !!props.include)

// The values filled in, per key; a key without one uses its default.
const values = ref<Record<string, string | boolean>>({})

const declared = computed(() => hasVariableProblems(props.rows) ? [] : props.rows)

function valueOf(row: VariableRow) {
  const key = row.key.trim()
  return key in values.value ? values.value[key] : row.value
}

function setValue(row: VariableRow, value: string | boolean) {
  values.value = { ...values.value, [row.key.trim()]: value }
}

function resetValues() {
  values.value = {}
}

function withValues(filled: boolean): Record<string, Variable> {
  const variables = toVariables(declared.value)
  if (filled) {
    for (const row of declared.value)
      variables[row.key.trim()].value = valueOf(row)
  }
  return variables
}

const result = ref<{ content: string, error?: string }>()
const defaultLines = ref(new Set<string>())
const isLoading = ref(false)

async function render() {
  isLoading.value = true
  try {
    const [filled, defaults] = await Promise.all([
      snippet.preview(props.content, withValues(true), props.fromPlugin),
      snippet.preview(props.content, withValues(false), props.fromPlugin),
    ])
    result.value = filled
    defaultLines.value = new Set(defaults.content.split('\n'))
  }
  catch {
    result.value = undefined
  }
  finally {
    isLoading.value = false
  }
}

watchDebounced([() => props.content, () => props.rows, values], render, { debounce: 300, deep: true, immediate: true })

// Lines that differ from the result with the default values.
const changed = computed(() => {
  const lines = new Set<number>()
  result.value?.content.split('\n').forEach((line, index) => {
    if (!defaultLines.value.has(line))
      lines.add(index + 1)
  })
  return lines
})

const undeclared = computed(() => {
  const keys = new Set(props.rows.map(r => r.key.trim()))
  return [...referencedKeys(props.content)].filter(key => !keys.has(key)).map(key => `.${key}`)
})

const hasChangedValues = computed(() => Object.keys(values.value).length > 0)

function optionsOf(row: VariableRow) {
  return row.options.map(o => ({ label: localize(o.labels, language.value) || o.value, value: o.value }))
}
</script>

<template>
  <DefineStatus>
    <AAlert
      v-if="undeclared.length > 0"
      type="warning"
      show-icon
      :title="$gettext('Not declared, so left without a value: %{names}', { names: undeclared.join(', ') })"
    />
    <AAlert
      v-if="result?.error"
      type="error"
      show-icon
      :title="$gettext('The snippet does not render as valid Nginx configuration.')"
      :description="result.error"
    />
    <AAlert
      v-else-if="result"
      type="success"
      show-icon
      :title="$gettext('The snippet renders as valid Nginx configuration.')"
    />
  </DefineStatus>

  <!-- Without variables the result is the content itself, so the directive
       that includes it is what matters. -->
  <div
    v-if="isIncludable"
    class="flex flex-col gap-3"
  >
    <div class="include-box">
      <ATypographyText
        code
        copyable
      >
        {{ include }}
      </ATypographyText>
    </div>
    <Status />
  </div>
  <div
    v-else
    class="grid grid-cols-1 gap-4 md:grid-cols-[minmax(0,17rem)_minmax(0,1fr)]"
  >
    <div class="form-card flex flex-col gap-3">
      <div class="hint text-xs font-semibold uppercase tracking-wide">
        {{ $gettext('What the site editor asks for') }}
      </div>
      <template v-if="declared.length > 0">
        <label
          v-for="row in declared"
          :key="row.id"
          class="flex flex-col gap-1"
        >
          <span class="hint text-[13px]">{{ localize(row.names, language) || row.key }}</span>
          <div
            v-if="row.type === 'boolean'"
            class="flex h-8 items-center"
          >
            <ASwitch
              :checked="valueOf(row) as boolean"
              @update:checked="setValue(row, $event as boolean)"
            />
          </div>
          <ASelect
            v-else-if="row.type === 'select'"
            :value="valueOf(row) as string"
            :options="optionsOf(row)"
            @update:value="setValue(row, $event as string)"
          />
          <AInput
            v-else
            :value="valueOf(row) as string"
            @update:value="setValue(row, $event)"
          />
        </label>
        <div>
          <AButton
            size="small"
            type="text"
            :disabled="!hasChangedValues"
            @click="resetValues"
          >
            <template #icon>
              <UndoOutlined />
            </template>
            {{ $gettext('Restore Default Values') }}
          </AButton>
        </div>
      </template>
      <div
        v-else
        class="hint text-sm"
      >
        {{ rows.length === 0 ? $gettext('Nothing to fill in.') : $gettext('Fix the variables to preview the snippet.') }}
      </div>
    </div>

    <div class="flex min-w-0 flex-col gap-2">
      <ASpin :spinning="isLoading && !result">
        <CodeView
          :content="result?.content ?? ''"
          :changed="hasChangedValues ? changed : undefined"
          class="min-h-40"
        />
      </ASpin>
      <Status />
    </div>
  </div>
</template>

<style scoped lang="less">
.include-box {
  padding: 12px 14px;
  border: 1px dashed var(--ant-color-border);
  border-radius: 8px;
  font-size: 15px;
}

.form-card {
  padding: 12px;
  border: 1px solid var(--ant-color-border-secondary);
  border-radius: 8px;
}

.hint {
  color: var(--ant-color-text-secondary);
}
</style>
