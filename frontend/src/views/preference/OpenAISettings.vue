<script setup lang="ts">
import {useGettext} from 'vue3-gettext'
import {IData} from '@/views/preference/typedef'
import {inject} from 'vue'

const {$gettext} = useGettext()

const data: IData = inject('data')!
</script>

<template>
  <a-form layout="vertical">
    <a-form-item :label="$gettext('ChatGPT Model')">
      <a-select v-model:value="data.openai.model">
        <a-select-option value="gpt-4-1106-preview">
          {{ $gettext('GPT-4-Turbo') }}
        </a-select-option>
        <a-select-option value="gpt-4">
          {{ $gettext('GPT-4') }}
        </a-select-option>
        <a-select-option value="gpt-4-32k">
          {{ $gettext('GPT-4-32K') }}
        </a-select-option>
        <a-select-option value="gpt-3.5-turbo">
          {{ $gettext('GPT-3.5-Turbo') }}
        </a-select-option>
      </a-select>
    </a-form-item>
    <a-form-item :label="$gettext('API Base Url')">
      <a-input v-model:value="data.openai.base_url"
               :placeholder="$gettext('Leave blank for the default: https://api.openai.com/')"/>
    </a-form-item>
    <a-form-item :label="$gettext('API Proxy')">
      <a-input v-model:value="data.openai.proxy" placeholder="http://127.0.0.1:1087"/>
    </a-form-item>
    <a-form-item :label="$gettext('API Token')">
      <a-input-password v-model:value="data.openai.token"/>
    </a-form-item>
  </a-form>
</template>

<style lang="less" scoped>

</style>
