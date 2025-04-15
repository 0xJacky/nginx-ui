<script setup lang="ts">
import type { Settings } from '@/api/settings'
import { LLM_MODELS, LLM_PROVIDERS } from '@/constants/llm'

const data: Ref<Settings> = inject('data') as Ref<Settings>
const errors: Record<string, Record<string, string>> = inject('errors') as Record<string, Record<string, string>>

const models = LLM_MODELS.map(model => ({
  value: model,
}))

const providers = LLM_PROVIDERS.map(provider => ({
  value: provider,
}))
</script>

<template>
  <AForm layout="vertical">
    <AFormItem
      :label="$gettext('Model')"
      :validate-status="errors?.openai?.model ? 'error' : ''"
      :help="errors?.openai?.model === 'safety_text'
        ? $gettext('The model name should only contain letters, unicode, numbers, hyphens, dashes, colons, and dots.')
        : ''"
    >
      <AAutoComplete
        v-model:value="data.openai.model"
        :options="models"
      />
    </AFormItem>
    <AFormItem
      :label="$gettext('API Base Url')"
      :validate-status="errors?.openai?.base_url ? 'error' : ''"
      :help="errors?.openai?.base_url === 'url'
        ? $gettext('The url is invalid.')
        : $gettext('To use a local large model, deploy it with ollama, vllm or lmdeploy. '
          + 'They provide an OpenAI-compatible API endpoint, so just set the baseUrl to your local API.')"
    >
      <AAutoComplete
        v-model:value="data.openai.base_url"
        :placeholder="$gettext('Leave blank for the default: https://api.openai.com/')"
        :options="providers"
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
      <AInputPassword v-model:value="data.openai.token" />
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
        :options="models"
      />
    </AFormItem>
  </AForm>
</template>

<style lang="less" scoped>

</style>
