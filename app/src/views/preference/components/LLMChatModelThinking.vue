<script setup lang="ts">
import type { LLMChatModel } from '@/api/llm'
import type { LLMThinkingLevel, LLMThinkingParams } from '@/constants/llm'
import { cloneDeep, isEqual } from 'lodash'
import { LLM_THINKING_LEVELS, LLM_THINKING_PRESETS, thinkingLevelLabel } from '@/constants/llm'

const models = defineModel<LLMChatModel[]>('models', { required: true })

const presetOptions = computed(() => LLM_THINKING_PRESETS.map(preset => ({
  label: preset.label(),
  value: preset.value,
})))

function presetParams(value: string) {
  return LLM_THINKING_PRESETS.find(preset => preset.value === value)?.params
}

function applyPreset(model: LLMChatModel, value: string) {
  model.thinking_preset = value
  const params = presetParams(value)
  // Custom keeps the current values as the starting point
  if (params)
    model.thinking_params = cloneDeep(params)
}

function configuredLevels(model: LLMChatModel) {
  return LLM_THINKING_LEVELS.filter(level => model.thinking_params?.[level])
}

const editingModel = ref<LLMChatModel>()
const draft = ref<Record<LLMThinkingLevel, string>>({ off: '', low: '', medium: '', high: '' })
const draftErrors = ref<Partial<Record<LLMThinkingLevel, string>>>({})

function openEditor(model: LLMChatModel) {
  editingModel.value = model
  draftErrors.value = {}
  draft.value = Object.fromEntries(LLM_THINKING_LEVELS.map(level => {
    const params = model.thinking_params?.[level]
    return [level, params ? JSON.stringify(params, null, 2) : '']
  })) as Record<LLMThinkingLevel, string>
}

function parseDraft(): LLMThinkingParams | undefined {
  const params: LLMThinkingParams = {}
  const errors: Partial<Record<LLMThinkingLevel, string>> = {}

  for (const level of LLM_THINKING_LEVELS) {
    const text = draft.value[level].trim()
    if (!text)
      continue

    try {
      const parsed = JSON.parse(text)
      if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed))
        errors[level] = $gettext('Enter a JSON object, such as {"reasoning_effort": "high"}')
      else
        params[level] = parsed
    }
    catch {
      errors[level] = $gettext('This is not valid JSON')
    }
  }

  draftErrors.value = errors
  return Object.keys(errors).length ? undefined : params
}

function saveEditor() {
  const model = editingModel.value
  const params = parseDraft()
  if (!model || !params)
    return

  model.thinking_params = params
  if (!isEqual(params, presetParams(model.thinking_preset)))
    model.thinking_preset = 'custom'

  editingModel.value = undefined
}
</script>

<template>
  <div class="flex flex-col gap-2">
    <div
      v-for="model in models"
      :key="model.name"
      class="flex flex-wrap items-center gap-x-3 gap-y-1"
    >
      <span class="min-w-40 flex-1 truncate font-mono text-[13px]">{{ model.name }}</span>
      <ASelect
        :value="model.thinking_preset"
        :options="presetOptions"
        size="small"
        class="w-72 max-w-full"
        :popup-match-select-width="false"
        @update:value="value => applyPreset(model, value as string)"
      />
      <AButton
        size="small"
        type="link"
        class="px-0"
        @click="openEditor(model)"
      >
        {{ configuredLevels(model).length
          ? configuredLevels(model).map(level => thinkingLevelLabel(level)).join(' · ')
          : $gettext('Set request fields') }}
      </AButton>
    </div>
  </div>

  <AModal
    :open="!!editingModel"
    :title="$gettext('Thinking levels of %{model}', { model: editingModel?.name ?? '' })"
    :ok-text="$gettext('Apply')"
    width="560px"
    @ok="saveEditor"
    @cancel="editingModel = undefined"
  >
    <p class="text-[var(--ant-color-text-secondary)]">
      {{ $gettext('Each level adds these fields to the chat request body, like extra_body in the OpenAI SDK. Leave a level empty to hide it in the assistant.') }}
    </p>
    <AForm layout="vertical">
      <AFormItem
        v-for="level in LLM_THINKING_LEVELS"
        :key="level"
        :label="thinkingLevelLabel(level)"
        :validate-status="draftErrors[level] ? 'error' : ''"
        :help="draftErrors[level]"
      >
        <ATextarea
          v-model:value="draft[level]"
          :auto-size="{ minRows: 1, maxRows: 6 }"
          class="font-mono text-[13px]"
          placeholder="{}"
        />
      </AFormItem>
    </AForm>
  </AModal>
</template>
