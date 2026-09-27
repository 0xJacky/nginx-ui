<script setup lang="ts">
import type { Cert } from '@/api/cert'
import { StdTable } from '@uozi-admin/curd'
import cert from '@/api/cert'
import certColumns from '@/views/certificate/CertificateList/certColumns'

// Certificate manager table (`GET /api/certs`) with row selection, shared by
// Change Certificate and the HTTPS card's "Existing certificate" method.

withDefaults(defineProps<{
  selectionType?: 'radio' | 'checkbox'
}>(), {
  selectionType: 'checkbox',
})

const selectedRows = defineModel<Cert[]>('selectedRows', { default: () => [] })

// The keys follow the rows, so clearing the rows also clears the table selection.
const selectedRowKeys = computed(() => selectedRows.value.map(row => row.id))

const columns = computed(() => certColumns.filter(item => item.pure))
</script>

<template>
  <StdTable
    v-model:selected-rows="selectedRows"
    :selected-row-keys="selectedRowKeys"
    :get-list-api="cert.getList"
    only-query
    disable-router-query
    :columns
    :row-selection-type="selectionType"
    :table-props="{
      rowKey: 'id',
    }"
  />
</template>
