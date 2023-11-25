<script setup lang="ts">
import {FileExclamationOutlined, FileTextOutlined} from '@ant-design/icons-vue'
import {computed, ref} from 'vue'
import {useRouter} from 'vue-router'

const props = defineProps(['ngx_config', 'current_server_idx', 'name'])

const accessIdx = ref()
const errorIdx = ref()

const hasAccessLog = computed(() => {
  let flag = false
  props.ngx_config.servers[props.current_server_idx].directives.forEach((v: any, k: any) => {
    if (v.directive === 'access_log') {
      flag = true
      accessIdx.value = k
      return
    }
  })
  return flag
})

const hasErrorLog = computed(() => {
  let flag = false
  props.ngx_config.servers[props.current_server_idx].directives.forEach((v: any, k: any) => {
    if (v.directive === 'error_log') {
      flag = true
      errorIdx.value = k
      return
    }
  })
  return flag
})

const router = useRouter()

function on_click_access_log() {
  router.push({
    path: '/nginx_log/site',
    query: {
      server_idx: props.current_server_idx,
      directive_idx: accessIdx.value,
      conf_name: props.name
    }
  })
}

function on_click_error_log() {
  router.push({
    path: '/nginx_log/site',
    query: {
      server_idx: props.current_server_idx,
      directive_idx: errorIdx.value,
      conf_name: props.name
    }
  })
}
</script>

<template>
  <a-space style="margin-left: -15px;margin-bottom: 5px" v-if="hasAccessLog||hasErrorLog">
    <a-button type="link" v-if="hasAccessLog" @click="on_click_access_log">
      <FileTextOutlined/>
      <translate>Access Logs</translate>
    </a-button>
    <a-button type="link" v-if="hasErrorLog" @click="on_click_error_log">
      <FileExclamationOutlined/>
      <translate>Error Logs</translate>
    </a-button>
  </a-space>
</template>

<style lang="less" scoped>

</style>
