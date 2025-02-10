<script setup lang="ts">
import user from '@/api/user'
import StdCurd from '@/components/StdDesign/StdDataDisplay/StdCurd.vue'
import userColumns from '@/views/user/userColumns'
</script>

<template>
  <StdCurd
    :scroll-x="1000"
    :title="$gettext('Manage Users')"
    :columns="userColumns"
    :api="user"
  />
</template>

<style scoped>

</style>
