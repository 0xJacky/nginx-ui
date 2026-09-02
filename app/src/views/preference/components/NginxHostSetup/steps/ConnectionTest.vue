<script setup lang="ts">
import type { SetupParams } from '@/api/host_setup'
import { LinkOutlined } from '@ant-design/icons-vue'
import { ref, watch } from 'vue'
import hostSetup from '@/api/host_setup'
import { getErrorMessage } from '@/lib/http'

const props = defineProps<{
  params: SetupParams
  trusted: boolean
}>()
const connected = defineModel<boolean>('connected', { default: false })

const isTestingConnection = ref(false)
const connectionDetail = ref('')
const connectionError = ref('')
// The step stays alive inside KeepAlive, so a stale response could otherwise
// mark a target as connected after the user changed it.
let testRequestID = 0

function resetResult() {
  testRequestID++
  connected.value = false
  connectionDetail.value = ''
  connectionError.value = ''
}

watch(
  [() => props.params.host_address, () => props.params.host_user, () => props.trusted],
  resetResult,
)

async function testConnection() {
  resetResult()
  const requestID = ++testRequestID
  isTestingConnection.value = true
  try {
    const response = await hostSetup.testConnection(props.params)
    if (requestID !== testRequestID)
      return
    connected.value = response.connected
    connectionDetail.value = response.detail
  }
  catch (error) {
    if (requestID !== testRequestID)
      return
    connectionError.value = getErrorMessage(error)
  }
  finally {
    // A stale run must not clear the loading state of a newer test.
    if (requestID === testRequestID)
      isTestingConnection.value = false
  }
}
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
