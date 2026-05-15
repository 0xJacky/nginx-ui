<script setup lang="ts">
import { SensitiveInput } from '@/components/SensitiveString'
import { LLM_MODELS, LLM_PROVIDER_BASE_URLS, LLM_PROVIDERS } from '@/constants/llm'
import useSystemSettingsStore from '../store'

const systemSettingsStore = useSystemSettingsStore()
const { data, errors } = storeToRefs(systemSettingsStore)

const modelOptions = LLM_MODELS.map(model => ({
  value: model,
}))

const providerOptions = LLM_PROVIDERS.map(provider => ({
  label: provider.label,
  value: provider.value,
}))

const baseUrlOptions = LLM_PROVIDER_BASE_URLS.map(baseUrl => ({
  value: baseUrl,
}))

const providerBaseUrlMap = LLM_PROVIDERS.reduce<Record<string, string>>((acc, provider) => {
  if (provider.baseUrl)
    acc[provider.value] = provider.baseUrl

  return acc
}, {})

const baseUrlPlaceholder = computed(() => {
  if (data.value?.openai.provider === 'atlas_cloud')
    return $gettext('Leave blank to use the Atlas Cloud endpoint: https://api.atlascloud.ai/v1')

  return $gettext('Leave blank for the default: https://api.openai.com/')
})

const baseUrlHelp = computed(() => {
  if (errors.value?.openai?.base_url === 'url')
    return $gettext('The url is invalid.')

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
  <AForm layout="vertical">
    <AFormItem
      :label="$gettext('Provider')"
      :validate-status="errors?.openai?.provider ? 'error' : ''"
    >
      <ASelect v-model:value="data.openai.provider">
        <ASelectOption
          v-for="provider in providerOptions"
          :key="provider.value"
          :value="provider.value"
        >
          {{ provider.label }}
        </ASelectOption>
      </ASelect>
    </AFormItem>
    <AFormItem
      :label="$gettext('Model')"
      :validate-status="errors?.openai?.model ? 'error' : ''"
      :help="errors?.openai?.model === 'safety_text'
        ? $gettext('The model name should only contain letters, unicode, numbers, hyphens, dashes, colons, and dots.')
        : ''"
    >
      <AAutoComplete
        v-model:value="data.openai.model"
        :options="modelOptions"
      />
    </AFormItem>
    <AFormItem
      :label="$gettext('API Base Url')"
      :validate-status="errors?.openai?.base_url ? 'error' : ''"
      :help="baseUrlHelp"
    >
      <AAutoComplete
        v-model:value="data.openai.base_url"
        :placeholder="baseUrlPlaceholder"
        :options="baseUrlOptions"
      />
    </AFormItem>
    <AFormItem
      :label="$gettext('API Proxy')"
      :validate-status="errors?.openai?.proxy ? 'error' : ''"
      :help="errors?.openai?.proxy === 'url'
        ? $gettext('The url is invalid.')
        : ''"
    >
      <AInput
        v-model:value="data.openai.proxy"
        placeholder="http://127.0.0.1:1087"
      />
    </AFormItem>
    <AFormItem
      :label="$gettext('API Token')"
      :validate-status="errors?.openai?.token ? 'error' : ''"
      :help="errors?.openai?.token === 'safety_text'
        ? $gettext('Token is not valid')
        : ''"
    >
      <SensitiveInput
        v-model="data.openai.token"
        path="openai.token"
      />
    </AFormItem>
    <AFormItem
      :label="$gettext('API Type')"
      :validate-status="errors?.openai?.apt_type ? 'error' : ''"
    >
      <ASelect v-model:value="data.openai.api_type">
        <ASelectOption value="OPEN_AI">
          OpenAI
        </ASelectOption>
        <ASelectOption value="AZURE">
          Azure
        </ASelectOption>
      </ASelect>
    </AFormItem>
    <AFormItem
      :label="$gettext('Enable Code Completion')"
    >
      <ASwitch v-model:checked="data.openai.enable_code_completion" />
    </AFormItem>
    <AFormItem
      v-if="data.openai.enable_code_completion"
      :label="$gettext('Code Completion Model')"
      :validate-status="errors?.openai?.code_completion_model ? 'error' : ''"
      :help="errors?.openai?.code_completion_model === 'safety_text'
        ? $gettext('The model name should only contain letters, unicode, numbers, hyphens, dashes, colons, and dots.')
        : $gettext('The model used for code completion, if not set, the chat model will be used.')"
    >
      <AAutoComplete
        v-model:value="data.openai.code_completion_model"
        :options="modelOptions"
      />
    </AFormItem>
  </AForm>
</template>

<style lang="less" scoped>

</style>
