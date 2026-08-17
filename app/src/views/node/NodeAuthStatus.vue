<script setup lang="ts">
import type { Node } from '@/api/node'
import {
  CheckCircleOutlined,
  ClockCircleOutlined,
  ExclamationCircleOutlined,
  LoadingOutlined,
  PauseCircleOutlined,
} from '@ant-design/icons-vue'
import nodeApi from '@/api/node'
import { formatDateTime } from '@/lib/helper'

const props = defineProps<{
  node: Node
}>()

const { message } = useGlobalApp()
const isRetrying = ref(false)
const retryState = shallowRef<Node>()
const nodeState = computed(() => retryState.value || props.node)

watch(() => props.node.auth_upgrade_status, status => {
  if (retryState.value?.auth_upgrade_status === status)
    retryState.value = undefined
})

const effectiveStatus = computed(() => {
  if (nodeState.value.auth_method === 'paired_ed25519')
    return nodeState.value.credential_status
  if (!nodeState.value.enabled)
    return 'paused'
  return nodeState.value.auth_upgrade_status || 'pending'
})

const statusPresentation = computed(() => {
  switch (effectiveStatus.value) {
    case 'active':
      return { color: 'green', label: $gettext('Paired signature'), icon: CheckCircleOutlined }
    case 'rotating':
      return { color: 'blue', label: $gettext('Rotating'), icon: LoadingOutlined }
    case 'unpaired':
      return { color: 'default', label: $gettext('Unpaired'), icon: ClockCircleOutlined }
    case 'revoked':
      return { color: 'red', label: $gettext('Revoked'), icon: ExclamationCircleOutlined }
    case 'in_progress':
      return { color: 'blue', label: $gettext('Upgrading'), icon: LoadingOutlined }
    case 'waiting_target':
      return { color: 'orange', label: $gettext('Waiting for target'), icon: ClockCircleOutlined }
    case 'failed':
      return { color: 'red', label: $gettext('Upgrade failed'), icon: ExclamationCircleOutlined }
    case 'paused':
      return { color: 'default', label: $gettext('Upgrade paused'), icon: PauseCircleOutlined }
    default:
      return { color: 'blue', label: $gettext('Upgrade pending'), icon: ClockCircleOutlined }
  }
})

const statusTitle = computed(() => {
  switch (effectiveStatus.value) {
    case 'in_progress':
      return $gettext('Authentication upgrade in progress')
    case 'waiting_target':
      return $gettext('Waiting for target node upgrade')
    case 'failed':
      return $gettext('Authentication upgrade failed')
    case 'paused':
      return $gettext('Authentication upgrade paused')
    default:
      return $gettext('Authentication upgrade pending')
  }
})

const isLegacyUpgrade = computed(() => nodeState.value.auth_method !== 'paired_ed25519')

const upgradeErrorMessage = computed(() => {
  switch (nodeState.value.auth_upgrade_error_code) {
    case 'timeout':
      return $gettext('The target node did not respond before the authentication upgrade timed out.')
    case 'connection_failed':
      return $gettext('The target node could not be reached. Check the node URL, network, and TLS settings.')
    case 'authentication_rejected':
      return $gettext('The target node rejected the saved Node Secret. Check and save the correct secret before retrying.')
    case 'target_rejected':
      return $gettext('The target node rejected the authentication upgrade request. Check the target node logs before retrying.')
    case 'invalid_response':
      return $gettext('The target node returned an invalid pairing response.')
    case 'invalid_confirmation':
      return $gettext('The target returned an invalid upgrade confirmation. Nginx UI stopped before accepting the new credential.')
    case 'persistence_failed':
      return $gettext('The paired credential could not be saved on this Nginx UI instance.')
    case 'missing_legacy_secret':
      return $gettext('The saved legacy Node Secret is unavailable. Save the Node Secret again before retrying.')
    default:
      return nodeState.value.auth_upgrade_error || $gettext('The authentication upgrade failed because of an internal error.')
  }
})

const statusDescription = computed(() => {
  switch (effectiveStatus.value) {
    case 'in_progress':
      return $gettext('The node is still connected with the legacy secret while Nginx UI switches this relationship to paired signatures.')
    case 'waiting_target':
      return $gettext('The target node does not support paired signatures yet. Upgrade the target node and Nginx UI will retry automatically.')
    case 'failed':
      return upgradeErrorMessage.value
    case 'paused':
      return $gettext('Enable the node to resume the authentication upgrade. The saved relationship has not been changed.')
    default:
      return $gettext('The legacy secret is saved and the authentication upgrade is queued. The current node connection remains available.')
  }
})

const stepItems = computed(() => [
  { title: $gettext('Legacy connection ready') },
  { title: $gettext('Request paired credential') },
  { title: $gettext('Verify target confirmation') },
  { title: $gettext('Save and switch authentication') },
])

const currentStep = computed(() => {
  switch (nodeState.value.auth_upgrade_step) {
    case 'verify':
      return 2
    case 'persist':
    case 'completed':
      return 3
    case 'request':
      return 1
    default:
      return 1
  }
})

const stepStatus = computed<'error' | 'process' | 'wait'>(() => {
  if (effectiveStatus.value === 'failed')
    return 'error'
  if (effectiveStatus.value === 'pending' || effectiveStatus.value === 'paused' || effectiveStatus.value === 'waiting_target')
    return 'wait'
  return 'process'
})
const canRetry = computed(() => effectiveStatus.value === 'failed' || effectiveStatus.value === 'waiting_target')

async function retryAuthenticationUpgrade() {
  isRetrying.value = true
  try {
    const updated = await nodeApi.retryAuthUpgrade(props.node.id)
    retryState.value = updated
    message.success($gettext('Authentication upgrade queued'))
  }
  finally {
    isRetrying.value = false
  }
}
</script>

<template>
  <ATag v-if="!isLegacyUpgrade" :color="statusPresentation.color" class="m-0">
    <component
      :is="statusPresentation.icon"
      :class="{ 'auth-upgrade-spinner': effectiveStatus === 'rotating' }"
    />
    {{ statusPresentation.label }}
  </ATag>

  <APopover v-else placement="rightTop" trigger="click">
    <template #title>
      <div class="flex items-center gap-2">
        <component
          :is="statusPresentation.icon"
          :class="{ 'auth-upgrade-spinner': effectiveStatus === 'in_progress' }"
        />
        <span>{{ statusTitle }}</span>
      </div>
    </template>

    <template #content>
      <div class="w-96 max-w-[calc(100vw-48px)]">
        <AAlert
          :type="effectiveStatus === 'failed' ? 'error' : effectiveStatus === 'waiting_target' ? 'warning' : 'info'"
          :message="statusDescription"
          show-icon
          class="mb-4"
        />

        <ASteps
          direction="vertical"
          size="small"
          :current="currentStep"
          :status="stepStatus"
          :items="stepItems"
        />

        <dl v-if="nodeState.auth_upgrade_attempted_at || nodeState.auth_upgrade_next_retry_at" class="mb-0 mt-3 grid grid-cols-[auto_1fr] gap-x-3 gap-y-1 text-sm">
          <template v-if="nodeState.auth_upgrade_attempted_at">
            <dt class="text-gray-500 dark:text-gray-400">
              {{ $gettext('Last attempt') }}
            </dt>
            <dd class="m-0">
              {{ formatDateTime(nodeState.auth_upgrade_attempted_at) }}
            </dd>
          </template>
          <template v-if="nodeState.auth_upgrade_next_retry_at && effectiveStatus !== 'paused'">
            <dt class="text-gray-500 dark:text-gray-400">
              {{ $gettext('Automatic retry after') }}
            </dt>
            <dd class="m-0">
              {{ formatDateTime(nodeState.auth_upgrade_next_retry_at) }}
            </dd>
          </template>
          <template v-if="nodeState.auth_upgrade_attempt_count">
            <dt class="text-gray-500 dark:text-gray-400">
              {{ $gettext('Attempts') }}
            </dt>
            <dd class="m-0">
              {{ nodeState.auth_upgrade_attempt_count }}
            </dd>
          </template>
        </dl>

        <details v-if="nodeState.auth_upgrade_error_code && (effectiveStatus === 'failed' || effectiveStatus === 'waiting_target')" class="mt-3 text-sm">
          <summary class="cursor-pointer select-none font-medium">
            {{ $gettext('Technical details') }}
          </summary>
          <code class="mt-2 block rounded bg-gray-100 px-3 py-2 text-xs dark:bg-gray-800">
            {{ nodeState.auth_upgrade_error_code }}
          </code>
        </details>

        <AButton
          v-if="canRetry"
          type="primary"
          size="small"
          class="mt-4"
          :loading="isRetrying"
          @click="retryAuthenticationUpgrade"
        >
          {{ $gettext('Retry authentication upgrade') }}
        </AButton>
      </div>
    </template>

    <button
      type="button"
      class="cursor-pointer border-0 bg-transparent p-0 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-500"
      :aria-label="statusTitle"
    >
      <ATag :color="statusPresentation.color" class="m-0">
        <component
          :is="statusPresentation.icon"
          :class="{ 'auth-upgrade-spinner': effectiveStatus === 'in_progress' }"
        />
        {{ statusPresentation.label }}
      </ATag>
    </button>
  </APopover>
</template>

<style scoped>
.auth-upgrade-spinner {
  animation: loadingCircle 1s infinite linear;
}

@media (prefers-reduced-motion: reduce) {
  .auth-upgrade-spinner {
    animation: none;
  }
}
</style>
