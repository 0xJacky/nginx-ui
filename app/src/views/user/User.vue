<script setup lang="ts">
import { StdCurd } from '@uozi-admin/curd'
import user from '@/api/user'
import userColumns from '@/views/user/userColumns'
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
