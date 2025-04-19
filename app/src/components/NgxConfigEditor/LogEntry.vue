<script setup lang="ts">
import type { NgxConfig } from '@/api/ngx'
import { FileExclamationOutlined, FileTextOutlined } from '@ant-design/icons-vue'

const props = defineProps<{
  ngxConfig: NgxConfig
  curServerIdx: number
  name?: string
}>()

const accessIdx = ref<number>()
const errorIdx = ref<number>()
const accessLogPath = ref<string>()
const errorLogPath = ref<string>()

const hasAccessLog = computed(() => {
  let flag = false
  props.ngxConfig?.servers[props.curServerIdx].directives?.forEach((v, k) => {
    if (v.directive === 'access_log') {
      flag = true
      accessIdx.value = k

      // Extract log path from directive params
      if (v.params) {
        const params = v.params.split(' ')
        if (params.length > 0) {
          accessLogPath.value = params[0]
        }
      }
    }
  })

  return flag
})

const hasErrorLog = computed(() => {
  let flag = false
  props.ngxConfig?.servers[props.curServerIdx].directives?.forEach((v, k) => {
    if (v.directive === 'error_log') {
      flag = true
      errorIdx.value = k

      // Extract log path from directive params
      if (v.params) {
        const params = v.params.split(' ')
        if (params.length > 0) {
          errorLogPath.value = params[0]
        }
      }
    }
  })

  return flag
})

const router = useRouter()

function onClickAccessLog() {
  router.push({
    path: '/nginx_log/site',
    query: {
      type: 'site',
      log_path: accessLogPath.value,
    },
  })
}

function onClickErrorLog() {
  router.push({
    path: '/nginx_log/site',
    query: {
      type: 'site',
      log_path: errorLogPath.value,
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
      @click="onClickAccessLog"
    >
      <FileTextOutlined />
      {{ $gettext('Access Logs') }}
    </AButton>
    <AButton
      v-if="hasErrorLog"
      type="link"
      @click="onClickErrorLog"
    >
      <FileExclamationOutlined />
      {{ $gettext('Error Logs') }}
    </AButton>
  </ASpace>
</template>

<style lang="less" scoped>

</style>
