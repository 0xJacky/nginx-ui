<script setup lang="ts">
import { StdCurd } from '@uozi-admin/curd'
import externalNotify from '@/api/external_notify'
import columns from '../components/ExternalNotify/columns'
</script>

<template>
  <StdCurd
    :title="$gettext('External Notify')"
    :columns="columns"
    :api="externalNotify"
    disable-view
    disable-export
    disable-trash
    disable-search
  />
</template>

<style scoped lang="less"></style>
