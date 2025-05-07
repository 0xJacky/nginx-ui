<script setup lang="ts">
import externalNotify from '@/api/external_notify'
import { StdCurd } from '@/components/StdDesign/StdDataDisplay'
import columns from '../components/ExternalNotify/columns'
</script>

<template>
  <StdCurd
    :title="$gettext('External Notify')"
    :columns="columns"
    :api="externalNotify"
    disable-view
    disable-query-params
  />
</template>

<style scoped lang="less"></style>
