<script setup lang="ts">
import type { NginxStatusResponse } from '@/api/ngx'
import type { HttpConfig } from '@/lib/http/types'
import { ReloadOutlined } from '@ant-design/icons-vue'
import { v4 as uuid } from 'uuid'
import ngx from '@/api/ngx'
import { NginxStatus } from '@/constants'
import { logLevel } from '@/constants/config'
import { useGlobalStore } from '@/pinia'
import { getRestartErrorMessage, isRestartOutcomeUnknown, shouldShowRestartError } from './restartRecovery'

const restartPollInterval = 750
const restartTimeout = 60_000
const restartRequestTimeout = 5_000

const global = useGlobalStore()
const { nginxStatus: status } = storeToRefs(global)
const { message } = App.useApp()
let isDisposed = false

function applyStatus(response: NginxStatusResponse) {
  if (response.control?.state === 'running' && response.control.action === 'restart') {
    status.value = NginxStatus.Restarting
  }
  else if (response.control?.state === 'running' && response.control.action === 'reload') {
    status.value = NginxStatus.Reloading
  }
  else if (response.running) {
    status.value = NginxStatus.Running
  }
  else {
    status.value = NginxStatus.Stopped
  }
}

async function getStatus(config?: HttpConfig) {
  const response = await ngx.status(config)
  applyStatus(response)

  return response
}

function reloadNginx() {
  status.value = NginxStatus.Reloading
  ngx.reload().then(r => {
    if (r.level < logLevel.Warn)
      message.success($gettext('Nginx reloaded successfully'))
    else if (r.level === logLevel.Warn)
      message.warning(r.message)
    else
      message.error(r.message)
  }).finally(() => getStatus())
}

async function restartNginx() {
  if (status.value === NginxStatus.Restarting)
    return

  const operationId = uuid()
  const previousStatus = status.value
  status.value = NginxStatus.Restarting
  try {
    await ngx.restart(operationId, {
      skipErrHandling: true,
      timeout: restartRequestTimeout,
    })
  }
  catch (error) {
    if (!isRestartOutcomeUnknown(error)) {
      if (shouldShowRestartError(error))
        message.error(getRestartErrorMessage(error) || $gettext('Failed to restart Nginx'))
      status.value = previousStatus
      await refreshStatusSilently()
      return
    }
  }

  await waitForRestart(operationId)
}

async function waitForRestart(operationId: string) {
  const deadline = Date.now() + restartTimeout
  let lastResponse: NginxStatusResponse | undefined

  while (Date.now() < deadline) {
    if (isDisposed)
      return

    await new Promise(resolve => setTimeout(resolve, restartPollInterval))
    if (isDisposed)
      return

    try {
      lastResponse = await getStatus({
        skipErrHandling: true,
        timeout: restartRequestTimeout,
      })
    }
    catch {
      continue
    }

    const operation = lastResponse.control
    if (!operation || operation.id !== operationId || operation.state === 'running')
      continue

    if (operation.state === 'failed') {
      message.error(operation.message || $gettext('Failed to restart Nginx'))
      return
    }

    if (operation.level < logLevel.Warn)
      message.success($gettext('Nginx restarted successfully'))
    else if (operation.level === logLevel.Warn && operation.message)
      message.warning(operation.message)
    else
      message.error(operation.message || $gettext('Failed to restart Nginx'))
    return
  }

  if (lastResponse) {
    status.value = lastResponse.running ? NginxStatus.Running : NginxStatus.Stopped
  }
  message.warning($gettext('Nginx restart status could not be confirmed. Please check the Nginx status.'))
}

async function refreshStatusSilently() {
  try {
    await getStatus({ skipErrHandling: true })
  }
  catch {
    // Keep the last known state when status cannot be refreshed.
  }
}

const visible = ref(false)

watch(visible, v => {
  if (v)
    getStatus()
})

onMounted(() => {
  getStatus()
})

onBeforeUnmount(() => {
  isDisposed = true
})
</script>

<template>
  <APopover
    v-model:open="visible"
    placement="bottomRight"
    @confirm="reloadNginx"
  >
    <template #content>
      <div class="content-wrapper">
        <h4>{{ $gettext('Nginx Control') }}</h4>
        <ABadge
          v-if="status === NginxStatus.Running"
          color="green"
          :text="$gettext('Running')"
        />
        <ABadge
          v-else-if="status === NginxStatus.Reloading"
          color="blue"
          :text="$gettext('Reloading')"
        />
        <ABadge
          v-else-if="status === NginxStatus.Restarting"
          color="orange"
          :text="$gettext('Restarting')"
        />
        <ABadge
          v-else
          color="red"
          :text="$gettext('Stopped')"
        />
      </div>
      <ASpace>
        <AButton
          size="small"
          type="link"
          :loading="status === NginxStatus.Restarting"
          :disabled="status === NginxStatus.Reloading"
          @click="restartNginx"
        >
          {{ $gettext('Restart') }}
        </AButton>
        <AButton
          size="small"
          type="link"
          :loading="status === NginxStatus.Reloading"
          :disabled="status === NginxStatus.Restarting"
          @click="reloadNginx"
        >
          {{ $gettext('Reload') }}
        </AButton>
      </ASpace>
    </template>
    <a>
      <ReloadOutlined />
    </a>
  </APopover>
</template>

<style lang="less" scoped>
a {
  color: #000000;
}

.dark {
  a {
    color: #fafafa;
  }
}

.content-wrapper {
  text-align: center;
  padding-top: 5px;
  padding-bottom: 5px;

  h4 {
    margin-bottom: 5px;
  }
}
</style>
