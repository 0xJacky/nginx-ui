<script setup lang="ts">
import { NginxStatus } from '@/constants'
import { useGlobalStore } from '@/pinia/moudule/global'

const global = useGlobalStore()
const { nginxStatus: status } = storeToRefs(global)
</script>

<template>
  <AAlert
    v-if="status !== NginxStatus.Running"
    class="mb-4"
    type="warning"
    show-icon
    :message="$gettext('Nginx is not running')"
  />
</template>

<style scoped lang="less">

</style>
