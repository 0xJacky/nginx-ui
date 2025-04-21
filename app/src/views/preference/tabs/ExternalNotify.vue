<script setup lang="ts">
import externalNotify from '@/api/external_notify'
import { StdCurd } from '@/components/StdDesign/StdDataDisplay'
import ExternalNotifyEditor from '../components/ExternalNotify'
import columns from '../components/ExternalNotify/columns'
</script>

<template>
  <StdCurd
    :title="$gettext('External Notify')"
    :columns="columns"
    :api="externalNotify"
    disable-view
    disable-query-params
  >
    <template #edit="{ data }">
      <ExternalNotifyEditor
        v-model="data.config"
        :type="data.type"
      />
    </template>
  </StdCurd>
</template>

<style scoped lang="less"></style>
