<script setup lang="ts">
import { inject } from 'vue'
import type { Settings } from '@/views/preference/typedef'

const data: Settings = inject('data') as Settings
const errors: Record<string, Record<string, string>> = inject('errors') as Record<string, Record<string, string>>
</script>

<template>
  <AForm layout="vertical">
    <AFormItem :label="$gettext('HTTP Host')">
      <p>{{ data.server.http_host }}</p>
    </AFormItem>
    <AFormItem :label="$gettext('HTTP Port')">
      <p>{{ data.server.http_port }}</p>
    </AFormItem>
    <AFormItem :label="$gettext('Run Mode')">
      <p>{{ data.server.run_mode }}</p>
    </AFormItem>
    <AFormItem :label="$gettext('Jwt Secret')">
      <p>{{ data.server.jwt_secret }}</p>
    </AFormItem>
    <AFormItem :label="$gettext('Node Secret')">
      <p>{{ data.server.node_secret }}</p>
    </AFormItem>
    <AFormItem :label="$gettext('Terminal Start Command')">
      <p>{{ data.server.start_cmd }}</p>
    </AFormItem>
    <AFormItem :label="$gettext('HTTP Challenge Port')">
      <AInputNumber v-model:value="data.server.http_challenge_port" />
    </AFormItem>
    <AFormItem
      :label="$gettext('Github Proxy')"
      :validate-status="errors?.server?.github_proxy ? 'error' : ''"
      :help="errors?.server?.github_proxy === 'url'
        ? $gettext('The url is not valid')
        : ''"
    >
      <AInput
        v-model:value="data.server.github_proxy"
        :placeholder="$gettext('For Chinese user: https://mirror.ghproxy.com/')"
      />
    </AFormItem>
    <AFormItem
      :label="$gettext('CADir')"
      :validate-status="errors?.server?.ca_dir ? 'error' : ''"
      :help="errors?.server?.ca_dir === 'url'
        ? $gettext('The url is not valid')
        : ''"
    >
      <AInput v-model:value="data.server.ca_dir" />
    </AFormItem>
  </AForm>
</template>

<style lang="less" scoped>

</style>
