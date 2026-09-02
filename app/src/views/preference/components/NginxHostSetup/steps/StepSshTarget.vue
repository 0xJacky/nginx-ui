<script setup lang="ts">
import type { SSHTarget } from '@/api/host_setup'
import { CheckCircleOutlined, ReloadOutlined } from '@antdv-next/icons'
import { computed, onActivated, onDeactivated, ref } from 'vue'
import hostSetup from '@/api/host_setup'
import { getErrorMessage } from '@/lib/http'
import { parseHostAddress } from '../hostAddress'
import { useHostSetupWizard } from '../useHostSetupWizard'
import { useLatestRequest } from '../useLatestRequest'
import AuthenticationMethod from './AuthenticationMethod.vue'

const {
  params,
  privateKeyOnce,
  publicKey,
  validatedPrivateKeyPath,
} = useHostSetupWizard()

const hostInput = computed({
  get: () => params.value.host_address,
  set: value => {
    const hostAddress = value.trim()
    params.value.host_address = hostAddress
    params.value.use_host_gateway = hostAddress.startsWith('host.docker.internal')
  },
})

const targets = ref<SSHTarget[]>([])
const { error: probeError, invalidate, isLoading: isProbingTargets, run } = useLatestRequest()

const reachableTargets = computed(() => targets.value.filter(target => target.reachable))
const isCurrentTargetReachable = computed(() =>
  reachableTargets.value.some(target => target.address === params.value.host_address))

async function probeTargets() {
  // Probe the address already entered too, so a non standard port is covered.
  await run(() => hostSetup.sshTargets(params.value.host_address), {
    onSuccess: response => {
      targets.value = response.targets
      // Only adopt a detected address when the operator has not typed one that
      // already answers, so a deliberate choice is never overwritten.
      const firstReachable = response.targets.find(target => target.reachable)
      if (firstReachable && !isCurrentTargetReachable.value && !params.value.host_address.trim())
        hostInput.value = firstReachable.address
    },
    onError: error => {
      targets.value = []
      probeError.value = getErrorMessage(error)
    },
  })
}

onActivated(() => {
  if (!targets.value.length)
    void probeTargets()
})

onDeactivated(invalidate)

const remoteWarning = computed(() => {
  if (params.value.access_mode !== 'mounted')
    return false
  // Bracketed IPv6 literals such as [::1]:22 must resolve to the address, not
  // to the opening bracket.
  const host = parseHostAddress(hostInput.value.trim()).host.trim()
  if (!host || host === 'host.docker.internal' || host === 'localhost' || host === '::1')
    return false
  if (/^127\./.test(host))
    return false
  if (/^172\.(?:1[6-9]|2\d|3[01])\.0\.1$/.test(host))
    return false
  return true
})
</script>

<template>
  <div class="space-y-4">
    <ACard size="small" :title="$gettext('SSH target')">
      <AFormItem :label="$gettext('Host address (host:port)')" required>
        <AInput v-model:value="hostInput" placeholder="host.docker.internal:22" />
        <template #extra>
          <ASpace orientation="vertical" size="small" class="w-full">
            <ASpace v-if="isCurrentTargetReachable" :size="6">
              <CheckCircleOutlined :style="{ color: '#52c41a' }" />
              <ATypographyText type="secondary" class="text-xs">
                {{ $gettext('This address answers on the SSH port.') }}
              </ATypographyText>
            </ASpace>
            <ASpace v-else-if="reachableTargets.length" orientation="vertical" size="small" class="w-full">
              <ATypographyText type="secondary" class="text-xs">
                {{ $gettext('These addresses answer on the SSH port from this container:') }}
              </ATypographyText>
              <ASpace wrap :size="6">
                <AButton
                  v-for="target in reachableTargets"
                  :key="target.address"
                  size="small"
                  @click="hostInput = target.address"
                >
                  {{ target.address }}
                </AButton>
              </ASpace>
            </ASpace>
            <ATypographyText v-else-if="probeError" type="danger" class="text-xs">
              {{ $gettext('Could not probe for reachable addresses: %{reason}', { reason: probeError }) }}
            </ATypographyText>
            <ATypographyText v-else-if="!isProbingTargets" type="secondary" class="text-xs">
              {{ $gettext('No address answered. Enter one manually, then use Probe again to test it.') }}
            </ATypographyText>
            <AButton type="link" size="small" :loading="isProbingTargets" @click="probeTargets">
              <ReloadOutlined />
              {{ $gettext('Probe again') }}
            </AButton>
          </ASpace>
        </template>
      </AFormItem>

      <AFormItem :label="$gettext('SSH user')" required>
        <AInput v-model:value="params.host_user" placeholder="nginxui" />
      </AFormItem>

      <AAlert
        v-if="remoteWarning"
        type="warning"
        show-icon
      >
        <template #title>
          {{ $gettext('Remote address detected') }}
        </template>
        <template #description>
          <p class="mb-2">
            {{ $gettext('This mode only supports nginx-ui and target nginx on the same host.') }}
            {{ $gettext('bind-mount cannot reach remote filesystems, so config editing, log viewing and certificate management will fail.') }}
          </p>
          <ATypographyLink
            href="/docs/guide/manage-multi-host-nginx-with-cluster.html"
            target="_blank"
            rel="noopener"
          >
            {{ $gettext('Use the cluster Node guide for cross-host setups.') }}
          </ATypographyLink>
        </template>
      </AAlert>
    </ACard>

    <AuthenticationMethod
      v-model:public-key="publicKey"
      v-model:private-key-once="privateKeyOnce"
      v-model:validated-private-key-path="validatedPrivateKeyPath"
      v-model:params="params"
    />
  </div>
</template>
