<script setup lang="ts">
import user from '@/api/user'
import userColumns from '@/views/user/userColumns'
import { StdCurd } from '@uozi-admin/curd'
</script>

<template>
  <StdCurd
    :scroll-x="1000"
    :title="$gettext('Manage Users')"
    :columns="userColumns"
    disable-export
    :api="user"
  />
</template>

<style scoped>

</style>
