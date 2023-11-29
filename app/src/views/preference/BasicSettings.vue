<script setup lang="ts">
import { useGettext } from 'vue3-gettext'
import { inject } from 'vue'
import type { IData } from '@/views/preference/typedef'

const { $gettext } = useGettext()
const data: IData = inject('data') as IData
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
    <AFormItem :label="$gettext('Github Proxy')">
      <AInput
        v-model:value="data.server.github_proxy"
        :placeholder="$gettext('Chinese user: https://ghproxy.com/')"
      />
    </AFormItem>
    <AFormItem :label="$gettext('CADir')">
      <AInput v-model:value="data.server.ca_dir" />
    </AFormItem>
  </AForm>
</template>

<style lang="less" scoped>

</style>
