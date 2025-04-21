<script setup lang="ts">
import useSystemSettingsStore from '../store'

const systemSettingsStore = useSystemSettingsStore()
const { data, errors } = storeToRefs(systemSettingsStore)
</script>

<template>
  <AForm layout="vertical">
    <AFormItem
      :label="$gettext('Github Proxy')"
      :validate-status="errors?.http?.github_proxy ? 'error' : ''"
      :help="errors?.http?.github_proxy === 'url'
        ? $gettext('The url is invalid')
        : ''"
    >
      <AInput
        v-model:value="data.http.github_proxy"
        :placeholder="$gettext('For Chinese user: https://mirror.ghproxy.com/')"
      />
    </AFormItem>
    <AFormItem :label="$gettext('Insecure Skip Verify')">
      <ATag :color="data.http.insecure_skip_verify ? 'green' : 'red'">
        {{ data.http.insecure_skip_verify ? $gettext('Enabled') : $gettext('Disabled') }}
      </ATag>
    </AFormItem>
  </AForm>
</template>

<style lang="less" scoped>
</style>
