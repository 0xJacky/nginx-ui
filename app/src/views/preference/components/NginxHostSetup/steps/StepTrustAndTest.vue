<script setup lang="ts">
import { useHostSetupWizard } from '../useHostSetupWizard'
import ConnectionTest from './ConnectionTest.vue'
import HostKeyTrust from './HostKeyTrust.vue'

const {
  isHostIdentityTrusted,
  isSSHConnected,
  params,
  requestParams,
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

    <ADivider title-placement="start">
      {{ $gettext('2. Test the SSH connection') }}
    </ADivider>

    <ConnectionTest
      v-model:connected="isSSHConnected"
      :params="requestParams"
      :trusted="isHostIdentityTrusted"
    />
  </div>
</template>
