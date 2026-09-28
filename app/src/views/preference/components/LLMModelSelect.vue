<script setup lang="ts">
import type { LLMModel } from '@/api/llm'
import type { LLMModelPreset } from '@/constants/llm'

interface ModelOption {
  label: string
  value: string
  isCustom?: boolean
  contextWindow?: number
}

interface ModelOptionGroup {
  label: string
  title: string
  options: ModelOption[]
}

const props = defineProps<{
  // Models discovered from the endpoint, undefined when there is no list
  models?: LLMModel[]
  // Provider presets offered when the endpoint gives no list
  presets?: LLMModelPreset[]
  loading?: boolean
  placeholder?: string
  allowClear?: boolean
  status?: '' | 'error'
  // Pick several models, value is then a list of model names
  multiple?: boolean
}>()

const value = defineModel<string | string[]>('value', { default: '' })

const searchValue = ref('')
const showAllModels = ref(false)

const presetByModel = computed(() => new Map((props.presets ?? []).map(preset => [preset.value, preset])))

function toOption(id: string): ModelOption {
  return {
    label: id,
    value: id,
    contextWindow: presetByModel.value.get(id)?.contextWindow,
  }
}

const chatModels = computed(() => props.models?.filter(model => model.kind === 'chat') ?? [])
const otherModels = computed(() => props.models?.filter(model => model.kind !== 'chat') ?? [])

const knownModelIds = computed(() => new Set([
  ...(props.models ?? []).map(model => model.id),
  ...presetByModel.value.keys(),
]))

const groups = computed(() => {
  const result: ModelOptionGroup[] = []
  const search = searchValue.value.trim()

  // Any name can be used, the endpoint may serve models it does not list
  if (search && !knownModelIds.value.has(search)) {
    result.push({
      label: $gettext('Custom model'),
      title: 'custom',
      options: [{ label: search, value: search, isCustom: true }],
    })
  }

  if (props.models) {
    result.push({
      label: $gettext('Chat models from the endpoint (%{count})', { count: String(chatModels.value.length) }),
      title: 'chat',
      options: chatModels.value.map(model => toOption(model.id)),
    })

    if (showAllModels.value && otherModels.value.length) {
      result.push({
        label: $gettext('Other models (embedding, audio, image)'),
        title: 'other',
        options: otherModels.value.map(model => toOption(model.id)),
      })
    }
  }
  else if (props.presets?.length) {
    result.push({
      label: $gettext('Presets'),
      title: 'presets',
      options: props.presets.map(preset => toOption(preset.value)),
    })
  }

  // Filtered here rather than by the Select, which keeps every option of a
  // group once the group itself passes its filter.
  return result
    .map(group => ({ ...group, options: group.options.filter(matchesSearch) }))
    .filter(group => group.options.length)
})

function matchesSearch(option: ModelOption) {
  const search = searchValue.value.trim().toLowerCase()
  return option.isCustom || !search || option.value.toLowerCase().includes(search)
}

function formatContextWindow(tokens: number) {
  if (tokens >= 1_000_000)
    return `${Number((tokens / 1_000_000).toFixed(1))}M`
  return `${Number((tokens / 1000).toFixed(1))}K`
}

function onSearch(search: string) {
  searchValue.value = search
}

function onSelect() {
  searchValue.value = ''
}

function toggleAllModels(event: MouseEvent) {
  event.preventDefault()
  showAllModels.value = !showAllModels.value
}
</script>

<template>
  <ASelect
    v-model:value="value"
    show-search
    :mode="multiple ? 'multiple' : undefined"
    :options="groups"
    :filter-option="false"
    :loading="loading"
    :placeholder="placeholder"
    :allow-clear="allowClear"
    :status="status"
    :not-found-content="loading ? $gettext('Fetching models...') : $gettext('Type a model name to use it')"
    :class="multiple ? 'w-full' : 'max-w-100'"
    @search="onSearch"
    @select="onSelect"
  >
    <template #optionRender="{ option }">
      <div
        v-if="option.data.isCustom"
        class="text-[var(--ant-color-primary)]"
      >
        {{ $gettext('Use custom model "%{model}"', { model: option.data.value }) }}
      </div>
      <div
        v-else
        class="flex items-center justify-between gap-2"
      >
        <span class="truncate font-mono text-[13px]">{{ option.data.value }}</span>
        <span
          v-if="option.data.contextWindow"
          class="shrink-0 text-xs text-[var(--ant-color-text-tertiary)]"
        >
          {{ $gettext('%{size} context', { size: formatContextWindow(option.data.contextWindow) }) }}
        </span>
      </div>
    </template>
    <template #popupRender="menu">
      <component :is="menu" />
      <div
        v-if="otherModels.length"
        class="mt-1 flex items-center justify-between gap-2 border-t border-t-solid border-[var(--ant-color-split)] px-3 pb-1 pt-2 text-xs text-[var(--ant-color-text-secondary)]"
        @mousedown.prevent
      >
        <span>
          {{ showAllModels
            ? $ngettext('Showing %{count} non-chat model', 'Showing %{count} non-chat models', otherModels.length, { count: String(otherModels.length) })
            : $ngettext('%{count} non-chat model hidden', '%{count} non-chat models hidden', otherModels.length, { count: String(otherModels.length) }) }}
        </span>
        <AButton
          type="link"
          size="small"
          class="px-0"
          @click="toggleAllModels"
        >
          {{ showAllModels ? $gettext('Hide') : $gettext('Show') }}
        </AButton>
      </div>
    </template>
  </ASelect>
</template>
