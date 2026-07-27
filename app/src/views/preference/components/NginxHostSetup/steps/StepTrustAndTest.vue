<script setup lang="ts">
import CheckPanel from '../CheckPanel.vue'
import { useHostSetupWizard } from '../useHostSetupWizard'
import ConnectionTest from './ConnectionTest.vue'
import HostKeyTrust from './HostKeyTrust.vue'

const {
  isHostIdentityTrusted,
  isSSHConnected,
  params,
} = useHostSetupWizard()

function invalidateConnection() {
  isSSHConnected.value = false
}
</script>

<template>
  <div>
    <HostKeyTrust
      v-model:trusted="isHostIdentityTrusted"
      :params="params"
      @invalidated="invalidateConnection"
    />

    <ADivider orientation="left">
      {{ $gettext('2. Test the SSH connection') }}
    </ADivider>

    <ConnectionTest
      v-model:connected="isSSHConnected"
      :params="params"
      :trusted="isHostIdentityTrusted"
    />

    <ADivider orientation="left">
      {{ $gettext('3. Check the connection') }}
    </ADivider>

    <CheckPanel
      group="connection"
      :title="$gettext('Connection checks')"
      :hint="$gettext('Verifies reachability, that the target is the container host, and that known_hosts is persisted.')"
      :disabled="!isSSHConnected"
    />
  </div>
</template>
