<script setup lang="ts">
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
  </div>
</template>
