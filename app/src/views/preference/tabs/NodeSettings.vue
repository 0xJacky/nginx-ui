<script setup lang="ts">
import SensitiveString from '@/components/SensitiveString'
import { SettingPanel, SettingRow } from '@/components/SettingPanel'
import useSystemSettingsStore from '../store'

const systemSettingsStore = useSystemSettingsStore()
const { data, errors } = storeToRefs(systemSettingsStore)
</script>

<template>
  <div v-if="data?.node">
    <SettingPanel :title="$gettext('Identity')">
      <SettingRow
        :title="$gettext('Node Secret')"
        path="node.secret"
        config-file="node"
      >
        <SensitiveString path="node.secret" :value="data.node.secret" />
      </SettingRow>
      <SettingRow
        :title="$gettext('Instance ID')"
        path="node.instance_id"
        config-file="node"
        :value="data.node.instance_id"
      />
      <SettingRow
        :title="$gettext('Node name')"
        :description="$gettext('Customize the name of local node to be displayed in the environment indicator.')"
        path="node.name"
        :error="errors?.node?.name
          ? $gettext('The node name should only contain letters, unicode, numbers, hyphens, dashes, colons, and dots.')
          : undefined"
      >
        <AInput v-model:value="data.node.name" class="w-60" />
      </SettingRow>
      <SettingRow
        :title="$gettext('Skip Installation')"
        path="node.skip_installation"
        config-file="node"
      >
        <ATag :color="data.node.skip_installation ? 'green' : 'red'">
          {{ data.node.skip_installation ? $gettext('Enabled') : $gettext('Disabled') }}
        </ATag>
      </SettingRow>
      <SettingRow
        :title="$gettext('Demo')"
        path="node.demo"
        config-file="node"
      >
        <ATag :color="data.node.demo ? 'green' : 'red'">
          {{ data.node.demo ? $gettext('Enabled') : $gettext('Disabled') }}
        </ATag>
      </SettingRow>
    </SettingPanel>

    <SettingPanel :title="$gettext('Footer')">
      <SettingRow
        :title="$gettext('ICP Number')"
        :description="$gettext('Shown in the page footer. Only needed for sites hosted in mainland China.')"
        path="node.icp_number"
        :error="errors?.node?.icp_number
          ? $gettext('The ICP Number should only contain letters, unicode, numbers, hyphens, dashes, colons, and dots.')
          : undefined"
      >
        <AInput
          v-model:value="data.node.icp_number"
          :placeholder="$gettext('For Chinese user')"
          class="w-60"
        />
      </SettingRow>
      <SettingRow
        :title="$gettext('Public Security Number')"
        :description="$gettext('Shown in the page footer. Only needed for sites hosted in mainland China.')"
        path="node.public_security_number"
        :error="errors?.node?.public_security_number
          ? $gettext('The Public Security Number should only contain letters, unicode, numbers, hyphens, dashes, colons, and dots.')
          : undefined"
      >
        <AInput
          v-model:value="data.node.public_security_number"
          :placeholder="$gettext('For Chinese user')"
          class="w-60"
        />
      </SettingRow>
    </SettingPanel>
  </div>
</template>
