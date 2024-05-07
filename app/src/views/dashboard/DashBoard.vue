<script setup lang="ts">
import ServerAnalytic from '@/views/dashboard/ServerAnalytic.vue'
import Environments from '@/views/dashboard/Environments.vue'

const key = ref(0)

setInterval(() => {
  key.value++
}, 5 * 60 * 1000)
</script>

<template>
  <div>
    <ServerAnalytic :key />
    <Environments />
  </div>
</template>

<style scoped lang="less">

</style>
