<script setup lang="ts">
import { ReloadOutlined } from '@ant-design/icons-vue'
import { message } from 'ant-design-vue'
import ngx from '@/api/ngx'
import { NginxStatus } from '@/constants'
import { useGlobalStore } from '@/pinia'
import { logLevel } from '@/views/config/constants'

const global = useGlobalStore()
const { nginxStatus: status } = storeToRefs(global)

async function getStatus() {
  const r = await ngx.status()
  if (r?.running === true)
    status.value = NginxStatus.Running
  else
    status.value = NginxStatus.Stopped

  return r
}

function reloadNginx() {
  status.value = NginxStatus.Reloading
  ngx.reload().then(r => {
    if (r.level < logLevel.Warn)
      message.success($gettext('Nginx reloaded successfully'))
    else if (r.level === logLevel.Warn)
      message.warn(r.message)
    else
      message.error(r.message)
  }).finally(() => getStatus())
}

async function restartNginx() {
  status.value = NginxStatus.Restarting
  await ngx.restart()

  getStatus().then(r => {
    if (r.level < logLevel.Warn)
      message.success($gettext('Nginx restarted successfully'))
    else if (r.level === logLevel.Warn)
      message.warn(r.message)
    else
      message.error(r.message)
  })
}

const visible = ref(false)

watch(visible, v => {
  if (v)
    getStatus()
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
          @click="restartNginx"
        >
          {{ $gettext('Restart') }}
        </AButton>
        <AButton
          size="small"
          type="link"
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
