<script setup lang="ts">
import type { DNSRecord, RecordListParams, RecordPayload } from '@/api/dns'
import { PlusOutlined, ReloadOutlined } from '@ant-design/icons-vue'
import { message } from 'ant-design-vue'
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import FooterToolBar from '@/components/FooterToolbar'
import { useDnsStore } from '@/pinia/moudule/dns'
import DNSRecordFilter from '@/views/dns/components/DNSRecordFilter.vue'
import DNSRecordForm from '@/views/dns/components/DNSRecordForm.vue'
import DNSRecordTable from '@/views/dns/components/DNSRecordTable.vue'

const route = useRoute()
const store = useDnsStore()
const router = useRouter()

const filters = ref<RecordListParams>({
  name: '',
  type: '',
})

const pagination = ref({
  current: 1,
  pageSize: 50,
  total: 0,
})

const pageSizeOptions = ['20', '50', '100', '200']

const domainId = computed(() => Number(route.params.id))

const isDrawerOpen = ref(false)
const editingRecord = ref<DNSRecord | null>(null)
const formModel = ref<RecordPayload>({
  type: 'A',
  name: '@',
  content: '',
  ttl: 600,
})

const showProxiedToggle = computed(() => {
  const provider = store.currentDomain?.dns_credential?.provider ?? ''
  return provider.toLowerCase().includes('cloudflare')
})

const contentSuggestions = computed(() => {
  const unique = new Set<string>()
  store.records.forEach(record => {
    const type = record.type?.toUpperCase?.() ?? ''
    if (record.content && (type === 'A' || type === 'CNAME')) {
      unique.add(record.content)
    }
  })
  return Array.from(unique)
})

const pageTitle = computed(() => {
  return store.currentDomain?.domain ?? $gettext('DNS Records')
})

async function initData() {
  await store.fetchDomainDetail(domainId.value)
  pagination.value.current = 1
  await fetchRecords()
}

async function fetchRecords() {
  await store.fetchRecords(domainId.value, {
    ...filters.value,
    page: pagination.value.current,
    per_page: pagination.value.pageSize,
  })
  const meta = store.recordsPagination
  pagination.value = {
    current: meta?.current_page ?? pagination.value.current,
    pageSize: meta?.per_page ?? pagination.value.pageSize,
    total: meta?.total ?? 0,
  }
}

function openCreateDrawer() {
  editingRecord.value = null
  formModel.value = {
    type: 'A',
    name: '@',
    content: '',
    ttl: 600,
  }
  isDrawerOpen.value = true
}

function openEditDrawer(record: DNSRecord) {
  editingRecord.value = record
  formModel.value = {
    type: record.type,
    name: record.name,
    content: record.content,
    ttl: record.ttl,
    priority: record.priority,
    weight: record.weight,
    proxied: record.proxied,
  }
  isDrawerOpen.value = true
}

async function handleSubmit() {
  if (editingRecord.value) {
    await store.updateRecord(domainId.value, editingRecord.value.id, formModel.value)
    message.success($gettext('Record updated'))
  }
  else {
    await store.createRecord(domainId.value, formModel.value)
    message.success($gettext('Record created'))
  }
  isDrawerOpen.value = false
}

async function handleDelete(record: DNSRecord) {
  await store.deleteRecord(domainId.value, record.id)
  message.success($gettext('Record deleted'))
}

function handleFilterSubmit() {
  pagination.value.current = 1
  fetchRecords()
}

function handlePageChange(page: number, pageSize: number) {
  pagination.value.current = page
  pagination.value.pageSize = pageSize
  fetchRecords()
}

function handlePageSizeChange(current: number, size: number) {
  pagination.value.current = current
  pagination.value.pageSize = size
  fetchRecords()
}

onMounted(() => {
  initData()
})

onBeforeUnmount(() => {
  store.resetRecords()
})
</script>

<template>
  <div class="record-manager">
    <ACard>
      <template #title>
        <ASpace align="center">
          {{ pageTitle }}
          <ATag v-if="store.currentDomain?.dns_credential?.provider">
            {{ store.currentDomain?.dns_credential?.provider }}
          </ATag>
        </ASpace>
      </template>
      <template #extra>
        <AButton type="link" size="small" @click="fetchRecords">
          <template #icon>
            <ReloadOutlined />
          </template>
          {{ $gettext('Refresh') }}
        </AButton>
        <AButton type="link" size="small" @click="openCreateDrawer">
          <template #icon>
            <PlusOutlined />
          </template>
          {{ $gettext('Add Record') }}
        </AButton>
      </template>

      <DNSRecordFilter v-model:filters="filters" @submit="handleFilterSubmit" />

      <DNSRecordTable
        class="mt-4"
        :records="store.records"
        :loading="store.recordsLoading"
        :show-proxied="showProxiedToggle"
        @edit="openEditDrawer"
        @delete="handleDelete"
      />
      <div class="mt-4 flex justify-end">
        <APagination
          :current="pagination.current"
          :page-size="pagination.pageSize"
          :total="pagination.total"
          :show-size-changer="true"
          :page-size-options="pageSizeOptions"
          @change="handlePageChange"
          @show-size-change="handlePageSizeChange"
        />
      </div>
    </ACard>

    <ADrawer
      :open="isDrawerOpen"
      :title="editingRecord ? $gettext('Edit Record') : $gettext('Create Record')"
      width="480"
      @close="isDrawerOpen = false"
    >
      <DNSRecordForm
        v-model:record="formModel"
        :show-proxied="showProxiedToggle"
        :value-suggestions="contentSuggestions"
      />
      <template #footer>
        <ASpace>
          <AButton @click="isDrawerOpen = false">
            {{ $gettext('Cancel') }}
          </AButton>
          <AButton type="primary" @click="handleSubmit">
            {{ $gettext('Save') }}
          </AButton>
        </ASpace>
      </template>
    </ADrawer>

    <FooterToolBar>
      <AButton @click="router.push('/dns/domains')">
        {{ $gettext('Back') }}
      </AButton>
    </FooterToolBar>
  </div>
</template>

<style scoped lang="less">
.record-manager {
  padding-bottom: 24px;
}
</style>
