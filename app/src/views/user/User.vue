<script setup lang="ts">
import StdCurd from '@/components/StdDesign/StdDataDisplay/StdCurd.vue'
import user from '@/api/user'
import userColumns from '@/views/user/userColumns'
</script>

<template>
  <StdCurd
    :title="$gettext('Manage Users')"
    :columns="userColumns"
    :api="user"
  />
</template>

<style scoped>

</style>
