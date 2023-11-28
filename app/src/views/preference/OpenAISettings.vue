<script setup lang="ts">
import { useGettext } from 'vue3-gettext'
import { inject } from 'vue'
import type { IData } from '@/views/preference/typedef'

const { $gettext } = useGettext()

const data: IData = inject('data')!
</script>

<template>
  <AForm layout="vertical">
    <AFormItem :label="$gettext('ChatGPT Model')">
      <ASelect v-model:value="data.openai.model">
        <ASelectOption value="gpt-4-1106-preview">
          {{ $gettext('GPT-4-Turbo') }}
        </ASelectOption>
        <ASelectOption value="gpt-4">
          {{ $gettext('GPT-4') }}
        </ASelectOption>
        <ASelectOption value="gpt-4-32k">
          {{ $gettext('GPT-4-32K') }}
        </ASelectOption>
        <ASelectOption value="gpt-3.5-turbo">
          {{ $gettext('GPT-3.5-Turbo') }}
        </ASelectOption>
      </ASelect>
    </AFormItem>
    <AFormItem :label="$gettext('API Base Url')">
      <AInput
        v-model:value="data.openai.base_url"
        :placeholder="$gettext('Leave blank for the default: https://api.openai.com/')"
      />
    </AFormItem>
    <AFormItem :label="$gettext('API Proxy')">
      <AInput
        v-model:value="data.openai.proxy"
        placeholder="http://127.0.0.1:1087"
      />
    </AFormItem>
    <AFormItem :label="$gettext('API Token')">
      <AInputPassword v-model:value="data.openai.token" />
    </AFormItem>
  </AForm>
</template>

<style lang="less" scoped>

</style>
