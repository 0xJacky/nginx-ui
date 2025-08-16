<script setup lang="tsx">
import { CloudUploadOutlined, SafetyCertificateOutlined } from '@ant-design/icons-vue'
import { StdTable } from '@uozi-admin/curd'
import cert from '@/api/cert'
import { useGlobalStore } from '@/pinia'
import WildcardCertificate from '../components/DNSIssueCertificate.vue'
import RemoveCert from '../components/RemoveCert.vue'
import certColumns from './certColumns'

const refWildcard = ref()
const refTable = ref()

const globalStore = useGlobalStore()

const { processingStatus } = storeToRefs(globalStore)
</script>

<template>
  <ACard :title="$gettext('Certificates')">
    <template #extra>
      <AButton
        type="link"
        size="small"
        @click="$router.push('/certificates/import')"
      >
        <CloudUploadOutlined />
        {{ $gettext('Import') }}
      </AButton>

      <AButton
        type="link"
        size="small"
        :disabled="processingStatus.auto_cert_processing"
        @click="() => refWildcard.open()"
      >
        <SafetyCertificateOutlined />
        {{ $gettext('Issue certificate') }}
      </AButton>
    </template>
    <StdTable
      ref="refTable"
      :api="cert"
      :columns="certColumns"
      :get-list-api="cert.getList"
      disable-view
      :scroll-x="1000"
      disable-delete
      @edit-item="record => $router.push(`/certificates/${record.id}`)"
    >
      <template #afterActions="{ record }">
        <RemoveCert
          :id="record.id"
          :certificate="record"
          :disabled="processingStatus.auto_cert_processing"
          @removed="() => refTable.refresh()"
        />
      </template>
    </StdTable>
    <WildcardCertificate
      ref="refWildcard"
      @issued="() => refTable.refresh()"
    />
  </ACard>
</template>

<style lang="less" scoped>

</style>
