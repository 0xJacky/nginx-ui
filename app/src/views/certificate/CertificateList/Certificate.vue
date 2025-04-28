<script setup lang="tsx">
import cert from '@/api/cert'
import StdTable from '@/components/StdDesign/StdDataDisplay/StdTable.vue'
import { useGlobalStore } from '@/pinia'
import { CloudUploadOutlined, SafetyCertificateOutlined } from '@ant-design/icons-vue'
import RemoveCert from '../components/RemoveCert.vue'
import WildcardCertificate from '../components/WildcardCertificate.vue'
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
        {{ $gettext('Issue wildcard certificate') }}
      </AButton>
    </template>
    <StdTable
      ref="refTable"
      :api="cert"
      :columns="certColumns"
      disable-view
      :scroll-x="1000"
      disable-delete
      @click-edit="id => $router.push(`/certificates/${id}`)"
    >
      <template #actions="{ record }">
        <RemoveCert
          :id="record.id"
          @removed="() => refTable.get_list()"
        />
      </template>
    </StdTable>
    <WildcardCertificate
      ref="refWildcard"
      @issued="() => refTable.get_list()"
    />
  </ACard>
</template>

<style lang="less" scoped>

</style>
