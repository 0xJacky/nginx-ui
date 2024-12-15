<script setup lang="ts">
import type { NgxConfig } from '@/api/ngx'
import { FileExclamationOutlined, FileTextOutlined } from '@ant-design/icons-vue'
import { useRouter } from 'vue-router'

const props = defineProps<{
  ngxConfig: NgxConfig
  currentServerIdx: number
  name?: string
}>()

const accessIdx = ref<number>()
const errorIdx = ref<number>()

const hasAccessLog = computed(() => {
  let flag = false
  props.ngxConfig?.servers[props.currentServerIdx].directives?.forEach((v, k) => {
    if (v.directive === 'access_log') {
      flag = true
      accessIdx.value = k
    }
  })

  return flag
})

const hasErrorLog = computed(() => {
  let flag = false
  props.ngxConfig?.servers[props.currentServerIdx].directives?.forEach((v, k) => {
    if (v.directive === 'error_log') {
      flag = true
      errorIdx.value = k
    }
  })

  return flag
})

const router = useRouter()

function on_click_access_log() {
  router.push({
    path: '/nginx_log/site',
    query: {
      server_idx: props.currentServerIdx,
      directive_idx: accessIdx.value,
      conf_name: props.name,
    },
  })
}

function on_click_error_log() {
  router.push({
    path: '/nginx_log/site',
    query: {
      server_idx: props.currentServerIdx,
      directive_idx: errorIdx.value,
      conf_name: props.name,
    },
  })
}
</script>

<template>
  <ASpace
    v-if="hasAccessLog || hasErrorLog"
    style="margin-left: -15px;margin-bottom: 5px"
  >
    <AButton
      v-if="hasAccessLog"
      type="link"
      @click="on_click_access_log"
    >
      <FileTextOutlined />
      {{ $gettext('Access Logs') }}
    </AButton>
    <AButton
      v-if="hasErrorLog"
      type="link"
      @click="on_click_error_log"
    >
      <FileExclamationOutlined />
      {{ $gettext('Error Logs') }}
    </AButton>
  </ASpace>
</template>

<style lang="less" scoped>

</style>
