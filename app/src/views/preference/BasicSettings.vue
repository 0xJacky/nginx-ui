<script setup lang="ts">
import type { Settings } from '@/api/settings'
import SensitiveString from '@/components/SensitiveString/SensitiveString.vue'

const data: Settings = inject('data') as Settings
const errors: Record<string, Record<string, string>> = inject('errors') as Record<string, Record<string, string>>
</script>

<template>
  <AForm layout="vertical">
    <AFormItem :label="$gettext('HTTP Host')">
      <p>{{ data.server.host }}</p>
    </AFormItem>
    <AFormItem :label="$gettext('HTTP Port')">
      <p>{{ data.server.port }}</p>
    </AFormItem>
    <AFormItem :label="$gettext('Run Mode')">
      <p>{{ data.server.run_mode }}</p>
    </AFormItem>
    <AFormItem :label="$gettext('Jwt Secret')">
      <SensitiveString :value="data.app.jwt_secret" />
    </AFormItem>
    <AFormItem :label="$gettext('Node Secret')">
      <SensitiveString :value="data.node.secret" />
    </AFormItem>
    <AFormItem :label="$gettext('Terminal Start Command')">
      <p>{{ data.terminal.start_cmd }}</p>
    </AFormItem>
    <AFormItem
      :label="$gettext('Node name')"
      :validate-status="errors?.node?.name ? 'error' : ''"
      :help="errors?.node?.name.includes('safety_text')
        ? $gettext('The node name should only contain letters, unicode, numbers, hyphens, dashes, colons, and dots.')
        : $gettext('Customize the name of local node to be displayed in the environment indicator.')"
    >
      <AInput v-model:value="data.node.name" />
    </AFormItem>
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
    <AFormItem
      :label="$gettext('ICP Number')"
      :validate-status="errors?.node?.icp_number ? 'error' : ''"
      :help="errors?.node?.icp_number.includes('safety_text')
        ? $gettext('The ICP Number should only contain letters, unicode, numbers, hyphens, dashes, colons, and dots.')
        : ''"
    >
      <AInput
        v-model:value="data.node.icp_number"
        :placeholder="$gettext('For Chinese user')"
      />
    </AFormItem>
    <AFormItem
      :label="$gettext('Public Security Number')"
      :validate-status="errors?.node?.public_security_number ? 'error' : ''"
      :help="errors?.node?.public_security_number.includes('safety_text')
        ? $gettext('The Public Security Number should only contain letters, unicode, numbers, hyphens, dashes, colons, and dots.')
        : ''"
    >
      <AInput
        v-model:value="data.node.public_security_number"
        :placeholder="$gettext('For Chinese user')"
      />
    </AFormItem>
  </AForm>
</template>

<style lang="less" scoped>

</style>
