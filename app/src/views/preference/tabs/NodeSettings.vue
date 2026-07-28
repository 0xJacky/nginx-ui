<script setup lang="ts">
import nodeApi from '@/api/node'
import SensitiveString from '@/components/SensitiveString'
import useSystemSettingsStore from '../store'

const systemSettingsStore = useSystemSettingsStore()
const { data, errors } = storeToRefs(systemSettingsStore)
const generatedPairingCode = ref('')
const generatedPairingCodeExpiresAt = ref('')

function createPairingCode() {
  nodeApi.createPairingCode().then(result => {
    generatedPairingCode.value = result.code
    generatedPairingCodeExpiresAt.value = result.expires_at
  })
}
</script>

<template>
  <AForm layout="vertical">
    <AFormItem :label="$gettext('Node Secret')">
      <SensitiveString path="node.secret" :value="data.node.secret" />
    </AFormItem>
    <AFormItem :label="$gettext('Instance ID')">
      <AInput :value="data.node.instance_id" readonly />
    </AFormItem>
    <AFormItem :label="$gettext('Pair Node')">
      <AButton @click="createPairingCode">
        {{ $gettext('Generate Pairing Code') }}
      </AButton>
    </AFormItem>
    <AFormItem :label="$gettext('Allow legacy node authentication')">
      <ASwitch v-model:checked="data.node.legacy_auth_enabled" />
      <AAlert
        v-if="data.node.legacy_auth_enabled"
        class="mt-3"
        type="warning"
        :message="$gettext('Disable this compatibility switch after every controller has been upgraded to paired signatures.')"
      />
    </AFormItem>
    <AFormItem :label="$gettext('Allow legacy Node Secret for MCP')">
      <ASwitch
        v-model:checked="data.node.legacy_mcp_auth_enabled"
        :disabled="!data.node.legacy_auth_enabled"
      />
      <AAlert
        v-if="data.node.legacy_mcp_auth_enabled"
        class="mt-3"
        type="warning"
        :message="$gettext('Temporary compatibility only. Replace MCP Node Secret clients with scoped service tokens.')"
      />
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

  <AModal
    :open="Boolean(generatedPairingCode)"
    :title="$gettext('One-time Pairing Code')"
    :footer="null"
    @cancel="generatedPairingCode = ''"
  >
    <ATypographyParagraph copyable>
      {{ generatedPairingCode }}
    </ATypographyParagraph>
    <p>{{ $gettext('Expires at: %{time}', { time: generatedPairingCodeExpiresAt }) }}</p>
  </AModal>
</template>

<style lang="less" scoped>
</style>
