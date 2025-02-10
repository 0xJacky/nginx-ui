<script setup lang="ts">
import type { ICP } from '@/api/public'
import publicApi from '@/api/public'

const icp = ref<ICP>({
  icp_number: '',
  public_security_number: '',
})

publicApi.getICP().then(r => {
  icp.value = r
})

const enabled = computed(() => {
  return icp.value.icp_number || icp.value.public_security_number
})

const showDot = computed(() => icp.value.icp_number && icp.value.public_security_number)

const publicSecurityNumberLink = computed(() =>
  `https://www.beian.gov.cn/portal/registerSystemInfo?recordcode=${icp.value.public_security_number}`)
</script>

<template>
  <div v-if="enabled">
    <a href="https://beian.miit.gov.cn/" target="_blank">{{ icp.icp_number }}</a>
    <span v-if="showDot"> · </span>
    <a v-if="icp.public_security_number" class="public_security_number" :href="publicSecurityNumberLink">
      <img src="//www.beian.gov.cn/img/new/gongan.png" alt="公安备案">
      <span class="ml-5">{{ icp.public_security_number }}</span></a>
  </div>
</template>

<style scoped lang="less">
a {
  font-size: 14px;
}

.public_security_number {
  position: relative;
  img {
    width: 16px;
    height: 16px;

    position: absolute;
    top: 50%;
    transform: translateY(-50%);
  }
}
</style>
