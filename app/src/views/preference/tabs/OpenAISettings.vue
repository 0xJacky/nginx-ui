<script setup lang="ts">
import type { SelectProps } from 'antdv-next'
import type { LLMChatModel } from '@/api/llm'
import { DownOutlined, ReloadOutlined, RightOutlined } from '@antdv-next/icons'
import { cloneDeep, isEqual } from 'lodash'
import llm from '@/api/llm'
import { SensitiveInput } from '@/components/SensitiveString'
import { SETTING_ROW_CONTEXT, SettingPanel, SettingRow } from '@/components/SettingPanel'
import { LLM_PROVIDER_BASE_URLS, LLM_PROVIDERS, suggestThinkingPreset } from '@/constants/llm'
import { translateError } from '@/lib/http/error'
import { normalizeHttpError } from '@/lib/http/normalizeError'
import LLMChatModelThinking from '../components/LLMChatModelThinking.vue'
import LLMModelSelect from '../components/LLMModelSelect.vue'
import { useLLMModelDiscovery } from '../composables/useLLMModelDiscovery'
import useSystemSettingsStore from '../store'

const systemSettingsStore = useSystemSettingsStore()
const { data, errors, isLoaded } = storeToRefs(systemSettingsStore)

// The models offered in the assistant. They are stored apart from the
// settings file and saved with this tab.
const chatModels = ref<LLMChatModel[]>([])
// Copy of the models as loaded or last saved, for the unsaved changes bar.
const savedChatModels = ref<LLMChatModel[]>([])

function setSavedChatModels(models: LLMChatModel[]) {
  chatModels.value = models
  savedChatModels.value = cloneDeep(models)
}

llm.get_chat_models().then(r => {
  setSavedChatModels(r.models ?? [])
})

const chatModelNames = computed({
  get: () => chatModels.value.map(model => model.name),
  set: (names: string[]) => {
    const existing = new Map(chatModels.value.map(model => [model.name, model]))
    chatModels.value = names.map(name => {
      const model = existing.get(name)
      if (model)
        return model

      const preset = suggestThinkingPreset(name, data.value.openai.base_url)
      return {
        name,
        thinking_preset: preset.value,
        thinking_params: cloneDeep(preset.params) ?? {},
      }
    })

    if (!names.includes(data.value.openai.model))
      data.value.openai.model = names[0] ?? ''
  },
})

const defaultModelOptions = computed(() => chatModelNames.value.map(name => ({
  label: name,
  value: name,
})))

onScopeDispose(systemSettingsStore.registerTabSaver('openai', {
  path: 'openai.chat_models',
  isDirty: () => !isEqual(chatModels.value, savedChatModels.value),
  discard: () => {
    chatModels.value = cloneDeep(savedChatModels.value)
  },
  save: async () => {
    try {
      const r = await llm.save_chat_models(chatModels.value)
      setSavedChatModels(r.models ?? [])
      return undefined
    }
    catch (err) {
      return translateError(normalizeHttpError(err))
    }
  },
}))

const openai = computed(() => data.value?.openai)
const {
  status: discoveryStatus,
  models: discoveredModels,
  errorMessage: discoveryError,
  refresh: refreshModels,
} = useLLMModelDiscovery(openai, isLoaded)

const discoveryAlert = computed(() => {
  switch (discoveryStatus.value) {
    case 'missing_token':
      return { type: 'info' as const, title: $gettext('Enter the API token to list the available models') }
    case 'loading':
      return { type: 'info' as const, title: $gettext('Fetching the model list...') }
    case 'success':
      return {
        type: 'success' as const,
        title: $ngettext(
          'Connected, %{count} model available',
          'Connected, %{count} models available',
          discoveredModels.value?.length ?? 0,
          { count: String(discoveredModels.value?.length ?? 0) },
        ),
      }
    case 'unsupported':
      return {
        type: 'warning' as const,
        title: discoveryError.value,
        description: $gettext('Type the model name below instead.'),
      }
    case 'unauthorized':
    case 'timeout':
    case 'error':
      return { type: 'error' as const, title: discoveryError.value }
    default:
      return undefined
  }
})

const hasAdvancedErrors = computed(() => !!(errors.value?.openai?.proxy || errors.value?.openai?.api_type))
const isAdvancedOpen = ref(false)
watch(hasAdvancedErrors, hasErrors => {
  if (hasErrors)
    isAdvancedOpen.value = true
}, { immediate: true })
watch(() => data.value?.openai.proxy, proxy => {
  if (proxy)
    isAdvancedOpen.value = true
}, { immediate: true })

// Open the advanced rows when the settings search points at one of them.
const settingRowContext = inject(SETTING_ROW_CONTEXT, undefined)
watch(() => settingRowContext?.highlightedPath.value, path => {
  if (path === 'openai.proxy' || path === 'openai.api_type')
    isAdvancedOpen.value = true
}, { immediate: true })

const modelHelp = computed(() => errors.value?.openai?.model === 'safety_text'
  ? $gettext('The model name should only contain letters, unicode, numbers, hyphens, dashes, colons, and dots.')
  : '')

const providerOptions: SelectProps['options'] = LLM_PROVIDERS.map(provider => ({
  label: provider.label,
  value: provider.value,
}))

const apiTypeOptions: SelectProps['options'] = [
  {
    label: 'OpenAI',
    value: 'OPEN_AI',
  },
  {
    label: 'Azure',
    value: 'AZURE',
  },
]

const baseUrlOptions = LLM_PROVIDER_BASE_URLS.map(baseUrl => ({
  value: baseUrl,
}))

const selectedProviderPreset = computed(() => LLM_PROVIDERS.find(
  provider => provider.value === data.value?.openai.provider,
))

const selectedModelPreset = computed(() => selectedProviderPreset.value?.models?.find(
  model => model.value === data.value?.openai.model,
))

const selectedModelCapabilities = computed(() => {
  const preset = selectedModelPreset.value
  if (!preset)
    return []

  return [
    `${$gettext('Context window')}: ${preset.contextWindow.toLocaleString()} ${$gettext('tokens')}`,
    `${$gettext('Input modalities')}: ${preset.inputModalities.map(formatCapability).join(', ')}`,
    `${$gettext('Thinking modes')}: ${preset.thinkingModes.map(formatCapability).join(', ')}`,
    `${$gettext('Pricing per million tokens')}: ${formatPricing()}`,
  ]
})

function filterBaseUrlOption(inputValue: string, option?: { value?: string }) {
  return option?.value?.toLowerCase().includes(inputValue.toLowerCase()) ?? false
}

function formatRegion(region: string) {
  if (region === 'global_en')
    return $gettext('Global')

  if (region === 'cn_zh')
    return $gettext('China')

  return region
}

function formatCapability(value: string) {
  if (value === 'always_on')
    return $gettext('Always on')

  return value.charAt(0).toUpperCase() + value.slice(1)
}

function formatPricing() {
  const pricing = selectedModelPreset.value?.pricing
  if (!pricing)
    return ''

  const prices = [
    `${$gettext('Input')}: $${pricing.input}`,
    `${$gettext('Output')}: $${pricing.output}`,
  ]

  if (pricing.cacheRead !== undefined)
    prices.push(`${$gettext('Cache read')}: $${pricing.cacheRead}`)

  if (pricing.cacheWrite !== undefined)
    prices.push(`${$gettext('Cache write')}: $${pricing.cacheWrite}`)

  return prices.join(' · ')
}

const providerBaseUrlMap = LLM_PROVIDERS.reduce<Record<string, string>>((acc, provider) => {
  if (provider.baseUrl)
    acc[provider.value] = provider.baseUrl

  return acc
}, {})

const baseUrlPlaceholder = computed(() => {
  if (data.value?.openai.provider === 'minimax')
    return $gettext('Leave blank to use the MiniMax global OpenAI-compatible endpoint: https://api.minimax.io/v1')

  if (data.value?.openai.provider === 'atlas_cloud')
    return $gettext('Leave blank to use the Atlas Cloud endpoint: https://api.atlascloud.ai/v1')

  return $gettext('Leave blank for the default: https://api.openai.com/')
})

const baseUrlHelp = computed(() => {
  if (errors.value?.openai?.base_url === 'url')
    return $gettext('The url is invalid.')

  if (data.value?.openai.provider === 'minimax') {
    return $gettext('MiniMax is OpenAI-compatible. Use https://api.minimax.io/v1 for global service or https://api.minimaxi.com/v1 for China service. For Anthropic-compatible clients, use https://api.minimax.io/anthropic or https://api.minimaxi.com/anthropic.')
  }

  if (data.value?.openai.provider === 'atlas_cloud') {
    return $gettext('Atlas Cloud is OpenAI-compatible. Use https://api.atlascloud.ai/v1 and an Atlas Cloud API key.')
  }

  return $gettext('To use a local large model, deploy it with ollama, vllm or lmdeploy. '
    + 'They provide an OpenAI-compatible API endpoint, so just set the baseUrl to your local API.')
})

watch(
  () => data.value?.openai.provider,
  (provider, previousProvider) => {
    if (!data.value || !provider)
      return

    const nextBaseUrl = providerBaseUrlMap[provider]
    if (!nextBaseUrl)
      return

    const currentBaseUrl = data.value.openai.base_url?.trim()
    const previousBaseUrl = previousProvider ? providerBaseUrlMap[previousProvider] : ''

    if (!currentBaseUrl || currentBaseUrl === previousBaseUrl)
      data.value.openai.base_url = nextBaseUrl
  },
)
</script>

<template>
  <div v-if="data?.openai">
    <SettingPanel :title="$gettext('Connection')">
      <SettingRow
        :title="$gettext('Provider')"
        path="openai.provider"
      >
        <ASelect
          v-model:value="data.openai.provider"
          :options="providerOptions"
          :status="errors?.openai?.provider ? 'error' : undefined"
          class="w-60"
        />
      </SettingRow>
      <SettingRow
        :title="$gettext('API Base Url')"
        :description="baseUrlHelp"
        path="openai.base_url"
        stacked
      >
        <AAutoComplete
          v-model:value="data.openai.base_url"
          :placeholder="baseUrlPlaceholder"
          :options="baseUrlOptions"
          :filter-option="filterBaseUrlOption"
          :default-active-first-option="false"
          :status="errors?.openai?.base_url ? 'error' : undefined"
          class="w-full"
        />
      </SettingRow>
      <AAlert
        v-if="selectedProviderPreset?.endpoints?.length"
        type="info"
        show-icon
        class="mb-3"
      >
        <template #title>
          {{ $gettext('Regional endpoints') }}
        </template>
        <template #description>
          <div
            v-for="endpoint in selectedProviderPreset.endpoints"
            :key="endpoint.region"
            class="mb-1 last:mb-0"
          >
            <strong>{{ formatRegion(endpoint.region) }}:</strong>
            OpenAI-compatible: {{ endpoint.openaiBaseUrl }} ·
            Anthropic-compatible: {{ endpoint.anthropicBaseUrl }} ·
            <a
              :href="endpoint.docsUrl"
              target="_blank"
              rel="noopener noreferrer"
            >
              {{ $gettext('Documentation') }}
            </a>
          </div>
        </template>
      </AAlert>
      <SettingRow
        :title="$gettext('API Token')"
        path="openai.token"
        :error="errors?.openai?.token === 'safety_text' ? $gettext('Token is not valid') : undefined"
      >
        <SensitiveInput
          v-model="data.openai.token"
          path="openai.token"
          class="w-80"
        />
      </SettingRow>
      <AAlert
        v-if="discoveryAlert"
        :type="discoveryAlert.type"
        :title="discoveryAlert.title"
        :description="discoveryAlert.description"
        show-icon
        class="mb-3"
      >
        <template
          v-if="discoveryStatus !== 'missing_token'"
          #action
        >
          <AButton
            size="small"
            type="link"
            :loading="discoveryStatus === 'loading'"
            @click="refreshModels"
          >
            <template #icon>
              <ReloadOutlined />
            </template>
            {{ discoveryStatus === 'success' ? $gettext('Refresh') : $gettext('Retry') }}
          </AButton>
        </template>
      </AAlert>
    </SettingPanel>

    <SettingPanel :title="$gettext('Models')">
      <SettingRow
        :title="$gettext('Assistant models')"
        :description="$gettext('The models to choose from in the assistant.')"
        path="openai.chat_models"
        stacked
      >
        <LLMModelSelect
          v-model:value="chatModelNames"
          multiple
          :models="discoveredModels"
          :presets="selectedProviderPreset?.models"
          :loading="discoveryStatus === 'loading'"
          :placeholder="$gettext('Choose one or more models')"
        />
      </SettingRow>
      <SettingRow
        :title="$gettext('Default model')"
        :description="$gettext('Used for new chats, chat titles and, unless set below, code completion.')"
        path="openai.model"
        :error="modelHelp || undefined"
      >
        <ASelect
          v-model:value="data.openai.model"
          :options="defaultModelOptions"
          :disabled="!defaultModelOptions.length"
          :placeholder="$gettext('Choose the assistant models first')"
          :status="errors?.openai?.model ? 'error' : undefined"
          class="w-60"
        />
      </SettingRow>
      <div
        v-if="selectedModelCapabilities.length"
        class="flex flex-wrap gap-1 pb-3"
      >
        <ATag
          v-for="capability in selectedModelCapabilities"
          :key="capability"
          class="m-0"
        >
          {{ capability }}
        </ATag>
      </div>
      <SettingRow
        v-if="chatModels.length"
        :title="$gettext('Thinking')"
        :description="$gettext('How each model switches its thinking levels. Pick the convention of your provider, or edit the request fields.')"
        path="openai.thinking"
        stacked
      >
        <LLMChatModelThinking v-model:models="chatModels" />
      </SettingRow>
    </SettingPanel>

    <SettingPanel :title="$gettext('Code Completion')">
      <SettingRow
        :title="$gettext('Enable Code Completion')"
        path="openai.enable_code_completion"
      >
        <ASwitch v-model:checked="data.openai.enable_code_completion" />
      </SettingRow>
      <SettingRow
        v-if="data.openai.enable_code_completion"
        :title="$gettext('Code Completion Model')"
        :description="$gettext('The model used for code completion, if not set, the chat model will be used.')"
        path="openai.code_completion_model"
        :error="errors?.openai?.code_completion_model === 'safety_text'
          ? $gettext('The model name should only contain letters, unicode, numbers, hyphens, dashes, colons, and dots.')
          : undefined"
        stacked
      >
        <LLMModelSelect
          v-model:value="data.openai.code_completion_model"
          :models="discoveredModels"
          :presets="selectedProviderPreset?.models"
          :loading="discoveryStatus === 'loading'"
          :placeholder="data.openai.model
            ? $gettext('Same as the chat model (%{model})', { model: data.openai.model })
            : $gettext('Same as the chat model')"
          :status="errors?.openai?.code_completion_model ? 'error' : ''"
          allow-clear
        />
      </SettingRow>
    </SettingPanel>

    <SettingPanel>
      <template #title>
        <AButton
          type="link"
          class="px-0"
          @click="isAdvancedOpen = !isAdvancedOpen"
        >
          <template #icon>
            <DownOutlined v-if="isAdvancedOpen" />
            <RightOutlined v-else />
          </template>
          {{ $gettext('Advanced') }}
        </AButton>
      </template>
      <template v-if="isAdvancedOpen">
        <SettingRow
          :title="$gettext('API Proxy')"
          path="openai.proxy"
          :error="errors?.openai?.proxy === 'url' ? $gettext('The url is invalid.') : undefined"
        >
          <AInput
            v-model:value="data.openai.proxy"
            placeholder="http://127.0.0.1:1087"
            class="w-80"
          />
        </SettingRow>
        <SettingRow
          :title="$gettext('API Type')"
          path="openai.api_type"
        >
          <ASelect
            v-model:value="data.openai.api_type"
            :options="apiTypeOptions"
            :status="errors?.openai?.api_type ? 'error' : undefined"
            class="w-60"
          />
        </SettingRow>
      </template>
    </SettingPanel>
  </div>
</template>
