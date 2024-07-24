<script setup lang="tsx">
import { CloudUploadOutlined, SafetyCertificateOutlined } from '@ant-design/icons-vue'
import certColumns from './certColumns'
import StdTable from '@/components/StdDesign/StdDataDisplay/StdTable.vue'
import cert from '@/api/cert'
import WildcardCertificate from '@/views/certificate/WildcardCertificate.vue'

const refWildcard = ref()
const refTable = ref()
</script>

<template>
  <ACard :title="$gettext('Certificates')">
    <template #extra>
      <AButton
        type="link"
        @click="$router.push('/certificates/import')"
      >
        <CloudUploadOutlined />
        {{ $gettext('Import') }}
      </AButton>

      <AButton
        type="link"
        @click="() => refWildcard.open()"
      >
        <SafetyCertificateOutlined />
        {{ $gettext('Issue wildcard certificate') }}
      </AButton>
    </template>
    <StdTable
      ref="refTable"
      :api="cert"
      :columns="certColumns"
      disable-view
      @click-edit="id => $router.push(`/certificates/${id}`)"
    />
    <WildcardCertificate
      ref="refWildcard"
      @issued="() => refTable.get_list()"
    />
  </ACard>
</template>

<style lang="less" scoped>

</style>
