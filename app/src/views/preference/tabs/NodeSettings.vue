<script setup lang="ts">
import SensitiveString from '@/components/SensitiveString'
import useSystemSettingsStore from '../store'

const systemSettingsStore = useSystemSettingsStore()
const { data, errors } = storeToRefs(systemSettingsStore)
</script>

<template>
  <AForm layout="vertical">
    <AFormItem :label="$gettext('Node Secret')">
      <SensitiveString :value="data.node.secret" />
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
    <AFormItem :label="$gettext('Skip Installation')">
      <ATag :color="data.node.skip_installation ? 'green' : 'red'">
        {{ data.node.skip_installation ? $gettext('Enabled') : $gettext('Disabled') }}
      </ATag>
    </AFormItem>
    <AFormItem :label="$gettext('Demo')">
      <ATag :color="data.node.demo ? 'green' : 'red'">
        {{ data.node.demo ? $gettext('Enabled') : $gettext('Disabled') }}
      </ATag>
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
