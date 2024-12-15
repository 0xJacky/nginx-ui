<script setup lang="ts">
import type { Settings } from '@/api/settings'

const data: Settings = inject('data')!
const errors: Record<string, Record<string, string>> = inject('errors') as Record<string, Record<string, string>>

const models = shallowRef([
  {
    value: 'gpt-4o-mini',
  },
  {
    value: 'gpt-4o',
  },
  {
    value: 'gpt-4-1106-preview',
  },
  {
    value: 'gpt-4',
  },
  {
    value: 'gpt-4-32k',
  },
  {
    value: 'gpt-3.5-turbo',
  },
])
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
        : $gettext('To use a local large model, deploy it with ollama, vllm or imdeploy. '
          + 'They provide an OpenAI-compatible API endpoint, so just set the baseUrl to your local API.')"
    >
      <AInput
        v-model:value="data.openai.base_url"
        :placeholder="$gettext('Leave blank for the default: https://api.openai.com/')"
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
  </AForm>
</template>

<style lang="less" scoped>

</style>
