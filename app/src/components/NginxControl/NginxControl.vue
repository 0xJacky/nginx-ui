<script setup lang="ts">
import { message } from 'ant-design-vue'
import { ReloadOutlined } from '@ant-design/icons-vue'
import ngx from '@/api/ngx'
import { logLevel } from '@/views/config/constants'
import { NginxStatus } from '@/constants'

const status = ref(0)
async function get_status() {
  const r = await ngx.status()
  if (r?.running === true)
    status.value = NginxStatus.Running
  else
    status.value = NginxStatus.Stopped

  return r
}

function reload_nginx() {
  status.value = NginxStatus.Reloading
  ngx.reload().then(r => {
    if (r.level < logLevel.Warn)
      message.success($gettext('Nginx reloaded successfully'))
    else if (r.level === logLevel.Warn)
      message.warn(r.message)
    else
      message.error(r.message)
  }).catch(e => {
    message.error(`${$gettext('Server error')} ${e?.message}`)
  }).finally(() => get_status())
}

async function restart_nginx() {
  status.value = NginxStatus.Restarting
  await ngx.restart()

  get_status().then(r => {
    if (r.level < logLevel.Warn)
      message.success($gettext('Nginx restarted successfully'))
    else if (r.level === logLevel.Warn)
      message.warn(r.message)
    else
      message.error(r.message)
  }).catch(e => {
    message.error(`${$gettext('Server error')} ${e?.message}`)
  })
}

const visible = ref(false)

watch(visible, v => {
  if (v)
    get_status()
})
</script>

<template>
  <APopover
    v-model:open="visible"
    placement="bottomRight"
    @confirm="reload_nginx"
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
          @click="restart_nginx"
        >
          {{ $gettext('Restart') }}
        </AButton>
        <AButton
          size="small"
          type="link"
          @click="reload_nginx"
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
