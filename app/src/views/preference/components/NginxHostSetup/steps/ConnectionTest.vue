<script setup lang="ts">
import type { SetupParams } from '@/api/host_setup'
import { LinkOutlined } from '@ant-design/icons-vue'
import { onDeactivated, ref, watch } from 'vue'
import hostSetup from '@/api/host_setup'
import { useLatestRequest } from '../useLatestRequest'

const props = defineProps<{
  params: SetupParams
  trusted: boolean
}>()
const connected = defineModel<boolean>('connected', { default: false })

// The step stays alive inside KeepAlive, so a stale response could otherwise
// mark a target as connected after the user changed it.
const { error: connectionError, invalidate, isLoading: isTestingConnection, reset, run } = useLatestRequest()
const connectionDetail = ref('')

function resetResult() {
  reset()
  connected.value = false
  connectionDetail.value = ''
}

watch(
  [() => props.params.host_address, () => props.params.host_user, () => props.trusted],
  resetResult,
)

async function testConnection() {
  resetResult()
  await run(() => hostSetup.testConnection(props.params), {
    onSuccess: response => {
      connected.value = response.connected
      connectionDetail.value = response.detail
    },
  })
}

onDeactivated(invalidate)
</script>

<template>
  <div class="space-y-4">
    <AAlert
      v-if="!trusted"
      type="info"
      show-icon
      :message="$gettext('Trust every presented host key before testing the connection.')"
    />

    <ASpace wrap>
      <AButton type="primary" :disabled="!trusted" :loading="isTestingConnection" @click="testConnection">
        <LinkOutlined />
        {{ $gettext('Test SSH connection') }}
      </AButton>
      <ATag v-if="connected" color="success" :bordered="false">
        {{ $gettext('Connected') }}
      </ATag>
    </ASpace>

    <AAlert
      v-if="connectionError"
      type="error"
      show-icon
      :message="$gettext('SSH connection failed')"
      :description="connectionError"
    />
    <AAlert
      v-if="connected"
      type="success"
      show-icon
      :message="$gettext('SSH connection succeeded')"
      :description="connectionDetail"
    />
    <AAlert
      v-else-if="connectionDetail"
      type="warning"
      show-icon
      :message="$gettext('SSH connection was not established')"
      :description="connectionDetail"
    />
  </div>
</template>
